package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kontroler-controller/internal/db"
	"kontroler-controller/internal/object"
	"kontroler-controller/internal/queue"
	"kontroler-controller/internal/webhook"
	"kontroler-controller/internal/workers"

	"github.com/google/uuid"
	pgxpool "github.com/jackc/pgx/v5/pgxpool"
	cron "github.com/robfig/cron/v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	var configPath string
	var pollDurationStr string
	flag.StringVar(&configPath, "configpath", "", "Path to worker config file (optional)")
	flag.StringVar(&pollDurationStr, "poll", "100ms", "task claim poll duration")
	flag.Parse()

	logger := ctrlzap.New(ctrlzap.UseDevMode(true))
	logf.SetLogger(logger)

	// Prepare context and signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Determine kube config
	var cfg *rest.Config
	var err error
	if cfgPath := os.Getenv("KUBECONFIG"); cfgPath != "" {
		cfg, err = clientcmd.BuildConfigFromFlags("", cfgPath)
	} else {
		cfg, err = rest.InClusterConfig()
	}
	if err != nil {
		logf.Log.Error(err, "failed to build kubeconfig")
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logf.Log.Error(err, "failed to create clientset")
		os.Exit(1)
	}

	// Configure DB manager
	var dbManager db.DBDAGManager
	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	switch os.Getenv("DB_TYPE") {
	case "postgresql":
		pgConfig, err := db.ConfigurePostgres()
		if err != nil {
			logf.Log.Error(err, "failed to configure postgres")
			os.Exit(1)
		}
		pool, err := pgxpool.NewWithConfig(context.Background(), pgConfig)
		if err != nil {
			logf.Log.Error(err, "failed to create postgres pool")
			os.Exit(1)
		}
		defer pool.Close()
		dbManager, err = db.NewPostgresDAGManagerWithMetrics(context.Background(), pool, &specParser)
		if err != nil {
			logf.Log.Error(err, "failed to create postgres dag manager")
			os.Exit(1)
		}
	default:
		// default to sqlite
		cfg, err := db.ConfigureSqlite()
		if err != nil {
			logf.Log.Error(err, "failed to configure sqlite")
			os.Exit(1)
		}
		dbm, dbConn, err := db.NewSqliteManagerWithMetrics(context.Background(), &specParser, cfg)
		if err != nil {
			logf.Log.Error(err, "failed to create sqlite manager")
			os.Exit(1)
		}
		defer dbConn.Close()
		dbManager = dbm
	}

	if err := dbManager.InitaliseDatabase(context.Background()); err != nil {
		logf.Log.Error(err, "failed to initialise database")
		os.Exit(1)
	}

	id := uuid.NewString()
	taskAllocator := workers.NewTaskAllocator(clientset, id)

	// Create a simple in-memory queue for the worker
	que := queue.NewMemoryQueue(context.Background())

	// Create a log store (filesystem by default)
	logStore, err := object.NewFileSystemLogStore("/tmp/kontroler-logs")
	if err != nil {
		logf.Log.Error(err, "failed to create log store (proceeding without log collection)")
		logStore = nil
	}

	// webhook channel
	webhookChan := make(chan webhook.WebhookPayload, 10)

	// parse poll duration
	pollDuration, err := time.ParseDuration(pollDurationStr)
	if err != nil {
		pollDuration = 100 * time.Millisecond
	}

	w := workers.NewWorker(que, logStore, webhookChan, dbManager, clientset, taskAllocator, pollDuration)

	// Start worker
	if err := w.Run(ctx); err != nil {
		logf.Log.Error(err, "worker stopped with error")
		os.Exit(1)
	}

	<-ctx.Done()
	fmt.Println("worker exiting")
}
