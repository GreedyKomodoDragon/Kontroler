package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/jackc/pgx/v5/pgxpool"
	cron "github.com/robfig/cron/v3"

	log "sigs.k8s.io/controller-runtime/pkg/log"

	kontrolerv1alpha1 "kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/config"
	"kontroler-controller/internal/controller"
	"kontroler-controller/internal/dag"
	"kontroler-controller/internal/db"
	"kontroler-controller/internal/object"
	kontrolerWebhook "kontroler-controller/internal/webhook"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(kontrolerv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var configPath string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&configPath, "configpath", "", "Path to configuration file")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	logger := zap.New(zap.UseFlagOptions(&opts))
	log.SetLogger(logger)

	ctrl.SetLogger(logger)

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancelation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	configController, err := config.ParseConfig(configPath)
	if err != nil {
		setupLog.Error(err, "failed to parse config")
		os.Exit(1)
	}

	namespaceConfigMap := map[string]cache.Config{}
	for _, namespace := range configController.Namespaces {
		namespaceConfigMap[strings.TrimSpace(namespace)] = cache.Config{}
	}

	// Get kubernetes config
	var config *rest.Config
	var kubeErr error
	if configController.KubeConfigPath != "" {
		config, kubeErr = clientcmd.BuildConfigFromFlags("", configController.KubeConfigPath)
	} else {
		config, kubeErr = rest.InClusterConfig()
	}
	if kubeErr != nil {
		setupLog.Error(kubeErr, "unable to get kubeconfig")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       configController.LeaderElectionID,
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
		Cache: cache.Options{
			DefaultNamespaces: namespaceConfigMap,
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Create the dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "failed to create api dynamicClient")
		os.Exit(1)
	}

	// Create a Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "failed to create api client")
		os.Exit(1)
	}

	// cronParser
	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	var dbDAGManager db.DBDAGManager

	switch os.Getenv("DB_TYPE") {
	case "postgresql":
		pgConfig, err := db.ConfigurePostgres()
		if err != nil {
			setupLog.Error(err, "failed to create postgres config")
			os.Exit(1)
		}

		pool, err := pgxpool.NewWithConfig(context.Background(), pgConfig)
		if err != nil {
			setupLog.Error(err, "failed to create postgres pool")
			os.Exit(1)
		}

		defer pool.Close()

		dbDAGManager, err = db.NewPostgresDAGManager(context.Background(), pool, &specParser)
		if err != nil {
			setupLog.Error(err, "failed to create postgres DAG manager")
			os.Exit(1)
		}
	case "sqlite":
		config, err := db.ConfigureSqlite()
		if err != nil {
			setupLog.Error(err, "failed to create sqlite config")
			os.Exit(1)
		}

		var dbConn *sql.DB
		dbDAGManager, dbConn, err = db.NewSqliteManager(context.Background(), &specParser, config)
		if err != nil {
			setupLog.Error(err, "failed to create sqlite DAG manager")
			os.Exit(1)
		}

		defer dbConn.Close()
	default:
		setupLog.Error(err, "unsupported DAG manager provided, 'postgresql' or 'sqlite'")
		os.Exit(1)
	}

	if err := dbDAGManager.InitaliseDatabase(context.Background()); err != nil {
		setupLog.Error(err, "failed to create DAG tables")
		os.Exit(1)
	}

	id, err := dbDAGManager.GetID(context.Background())
	if err != nil {
		setupLog.Error(err, "failed to create DAG tables")
		os.Exit(1)
	}

	logStore, err := object.NewLogStore()
	if err != nil {
		setupLog.Error(err, "failed to connect to object store")
		os.Exit(1)
	}

	if logStore == nil {
		setupLog.Info("log collection not enabled for s3")
	}

	webhookChannel := make(chan kontrolerWebhook.WebhookPayload, 10)
	webhookManager := kontrolerWebhook.NewWebhookManager(webhookChannel)

	// Create a cancellable context
	webhookContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the webhook manager in a separate goroutine
	go func() {
		if err := webhookManager.Listen(webhookContext); err != nil {
			setupLog.Error(err, "webhook manager stopped")
		}
	}()

	taskAllocator := dag.NewTaskAllocator(clientset, id)
	watchers := make([]dag.TaskWatcher, len(configController.Namespaces))

	for i, namespace := range configController.Namespaces {
		namespaceTrimmed := strings.TrimSpace(namespace)

		taskWatcher, err := dag.NewTaskWatcher(namespaceTrimmed, clientset, taskAllocator,
			dbDAGManager, id, logStore, webhookChannel)
		if err != nil {
			setupLog.Error(err, "failed to create task watcher", "namespace", namespaceTrimmed)
			os.Exit(1)
		}

		watchers[i] = taskWatcher
	}

	taskScheduler := dag.NewDagScheduler(dbDAGManager, dynamicClient)

	closeChannels := make([]chan struct{}, len(configController.Namespaces))
	for i := 0; i < len(configController.Namespaces); i++ {
		closeChan := make(chan struct{})
		closeChannels[i] = closeChan
	}

	if err = (&controller.DAGReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		DbManager: dbDAGManager,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DAG")
		os.Exit(1)
	}
	if err = (&controller.DagRunReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		DbManager:     dbDAGManager,
		TaskAllocator: taskAllocator,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DagRun")
		os.Exit(1)
	}

	if err = (&kontrolerv1alpha1.DagRun{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "DagRun")
		os.Exit(1)
	}

	if err = (&controller.DagTaskReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		DbManager: dbDAGManager,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DagTask")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	stopCh := ctrl.SetupSignalHandler()

	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		log.Log.Info("Became the leader, starting the controller.")
		go taskScheduler.Run(ctx)

		for i := 0; i < len(configController.Namespaces); i++ {
			go watchers[i].StartWatching(closeChannels[i])
		}

		// Watch for context cancellation (graceful shutdown when leadership is lost or signal is caught)
		<-ctx.Done()

		log.Log.Info("Losing leadership or shutting down, cleaning up...")
		return nil
	})); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Create a channel to listen for OS signals
	quit := make(chan os.Signal, 1)

	// syscall.SIGTERM is for kubernetes
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	setupLog.Info("starting manager")
	if err := mgr.Start(stopCh); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	// Wait for OS signal to gracefully shutdown the server
	<-quit
	log.Log.Info("shutting controller down")

	// Shutdown the server gracefully
	for i := 0; i < len(closeChannels); i++ {
		close(closeChannels[i])
	}

	log.Log.Info("Controller gracefully stopped")
}
