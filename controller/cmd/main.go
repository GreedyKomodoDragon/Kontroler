package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	v1 "k8s.io/api/core/v1"
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
	_ "kontroler-controller/internal/metrics"
	"kontroler-controller/internal/object"
	"kontroler-controller/internal/queue"
	kontrolerWebhook "kontroler-controller/internal/webhook"
	"kontroler-controller/internal/workers"
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
	var tlsCertDir string
	var tlsCertName string
	var tlsKeyName string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&configPath, "configpath", "", "Path to configuration file")
	flag.StringVar(&tlsCertDir, "tls-cert-dir", "", "Directory containing TLS certificates for secure metrics endpoint")
	flag.StringVar(&tlsCertName, "tls-cert-name", "tls.crt", "Name of the TLS certificate file")
	flag.StringVar(&tlsKeyName, "tls-key-name", "tls.key", "Name of the TLS private key file")
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

	// Configure metrics server options
	metricsOpts := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	// If TLS is enabled for metrics and certificate directory is provided, configure it
	if secureMetrics && tlsCertDir != "" {
		// Validate that the certificate files exist
		certPath := filepath.Join(tlsCertDir, tlsCertName)
		keyPath := filepath.Join(tlsCertDir, tlsKeyName)

		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			setupLog.Error(err, "TLS certificate file not found", "path", certPath)
			os.Exit(1)
		}

		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			setupLog.Error(err, "TLS private key file not found", "path", keyPath)
			os.Exit(1)
		}

		metricsOpts.CertDir = tlsCertDir
		metricsOpts.CertName = tlsCertName
		metricsOpts.KeyName = tlsKeyName
	} else if secureMetrics {
		setupLog.Info("TLS enabled for metrics endpoint with auto-generated certificates")
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	configController, err := config.ParseConfig(configPath)
	if err != nil {
		setupLog.Error(err, "failed to parse config")
		os.Exit(1)
	}

	// Create map of unique namespaces from worker configs
	namespaceConfigMap := map[string]cache.Config{}
	for _, worker := range configController.Workers.Workers {
		namespaceConfigMap[strings.TrimSpace(worker.Namespace)] = cache.Config{}
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
		Scheme:                 scheme,
		Metrics:                metricsOpts,
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

		dbDAGManager, err = db.NewPostgresDAGManagerWithMetrics(context.Background(), pool, &specParser)
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
		dbDAGManager, dbConn, err = db.NewSqliteManagerWithMetrics(context.Background(), &specParser, config)
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

	logStore, err := createLogStore(configController.LogStore)
	if err != nil {
		setupLog.Error(err, "failed to connect to object store")
		os.Exit(1)
	}

	if logStore == nil {
		setupLog.Info("log collection not enabled for s3")
	}

	// Create root context with cancellation
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// Create a WaitGroup to track all running goroutines
	var wg sync.WaitGroup

	// Setup graceful shutdown
	shutdown := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		setupLog.Info("shutdown signal received")
		rootCancel() // Cancel root context
		close(shutdown)
	}()

	// Create webhook context as child of root context
	webhookChannel := make(chan kontrolerWebhook.WebhookPayload, 10)
	webhookManager := kontrolerWebhook.NewWebhookManager(webhookChannel)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := webhookManager.Listen(rootCtx); err != nil {
			setupLog.Error(err, "webhook manager stopped")
		}
	}()

	taskAllocator := workers.NewTaskAllocator(clientset, id)
	var totalWorkers int
	for _, workerConfig := range configController.Workers.Workers {
		totalWorkers += workerConfig.Count
	}

	// Initialize slices to hold watchers and workers
	watchers := make([]dag.TaskWatcher, len(configController.Workers.Workers))
	eventWatchers := make([]dag.EventWatcher, len(configController.Workers.Workers))
	wrkers := make([]workers.Worker[*v1.Pod], totalWorkers)
	closeChannels := make([]chan struct{}, len(configController.Workers.Workers))
	closeEventChannels := make([]chan struct{}, len(configController.Workers.Workers))

	// Initialize workers and watchers based on config
	currentIndex := 0
	for i, workerConfig := range configController.Workers.Workers {
		i := i // capture range variable

		queues := make([]queue.Queue, workerConfig.Count)

		pollDuration, err := time.ParseDuration(configController.Workers.PollDuration)
		if err != nil {
			setupLog.Error(err, "invalid poll duration", "duration", configController.Workers.PollDuration)
			os.Exit(1)
		}

		for j := 0; j < workerConfig.Count; j++ {
			var que queue.Queue
			var err error

			// Generate unique worker ID
			workerID := fmt.Sprintf("%s-worker-%d", workerConfig.Namespace, j)

			switch configController.Workers.WorkerType {
			case "memory":
				que = queue.NewMemoryQueue(context.Background())
			case "pebble":
				queuePath := filepath.Join(configController.Workers.QueueDir, workerID)
				que, err = queue.NewPebbleQueue(rootCtx, queuePath, workerConfig.Namespace)
				if err != nil {
					setupLog.Error(err, "failed to create pebble queue",
						"worker_id", workerID,
						"namespace", workerConfig.Namespace)
					os.Exit(1)
				}
			default:
				setupLog.Error(err, "unsupported worker type provided, 'memory' or 'pebble'")
				os.Exit(1)
			}
			queues[j] = que

			wrkers[currentIndex] = workers.NewWorker(que, logStore, webhookChannel,
				dbDAGManager, clientset, taskAllocator, pollDuration)
			currentIndex++
		}

		// create listeners
		eventHandler := workers.NewPodEventHandler(queues)

		taskWatcher, err := dag.NewTaskWatcher(id, workerConfig.Namespace, clientset, eventHandler)
		if err != nil {
			setupLog.Error(err, "failed to create task watcher", "namespace", workerConfig.Namespace)
			os.Exit(1)
		}

		eventListener := workers.NewEventHandler(queues, clientset)
		eventWatcher, err := dag.NewEventWatcher(id, workerConfig.Namespace, clientset, eventListener)
		if err != nil {
			setupLog.Error(err, "failed to create event watcher", "namespace", workerConfig.Namespace)
			os.Exit(1)
		}

		watchers[i] = taskWatcher
		eventWatchers[i] = eventWatcher
		closeChannels[i] = make(chan struct{})
		closeEventChannels[i] = make(chan struct{})
	}

	taskScheduler := dag.NewDagScheduler(dbDAGManager, dynamicClient)

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
		LogStore:      logStore,
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

	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		log.Log.Info("Became the leader, starting the controller.")

		wg.Add(1)
		go func() {
			defer wg.Done()
			taskScheduler.Run(ctx)
		}()

		// Start the task watchers and workers
		currentIndex := 0
		for i, workerConfig := range configController.Workers.Workers {
			i := i // capture range variable

			wg.Add(2)
			go func() {
				defer wg.Done()
				watchers[i].StartWatching(closeChannels[i])
			}()

			go func() {
				defer wg.Done()
				eventWatchers[i].StartWatching(closeEventChannels[i])
			}()

			for j := 0; j < workerConfig.Count; j++ {
				worker := wrkers[currentIndex]
				if err := worker.Queue().Start(); err != nil {
					setupLog.Error(err, "failed to start queue", "worker_id", worker.ID())
					os.Exit(1)
				}

				wg.Add(1)
				go func() {
					defer wg.Done()
					// Start the worker
					worker.Run(ctx)
				}()
				currentIndex++
			}
		}

		<-ctx.Done()
		log.Log.Info("Leadership lost or context cancelled, initiating cleanup...")
		return nil
	})); err != nil {
		setupLog.Error(err, "unable to add controller runnable")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	go func() {
		if err := mgr.Start(rootCtx); err != nil {
			setupLog.Error(err, "problem running manager")
			rootCancel()
		}
	}()

	// Wait for shutdown signal
	<-shutdown
	setupLog.Info("initiating graceful shutdown")

	// Close all watcher channels
	for i := 0; i < len(configController.Workers.Workers); i++ {
		close(closeChannels[i])
	}

	// Wait for all goroutines to finish
	wg.Wait()
	setupLog.Info("graceful shutdown completed")
}

func createLogStore(logStoreConfig config.LogStore) (object.LogStore, error) {
	switch logStoreConfig.StoreType {
	case "filesystem":
		return object.NewFileSystemLogStore(logStoreConfig.FileSystem.BaseDir)
	case "s3":
		return object.NewLogStore()
	default:
		return nil, fmt.Errorf("unsupported log store type: %s", logStoreConfig.StoreType)
	}
}
