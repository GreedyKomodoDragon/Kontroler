package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"flag"
	"kontroler-server/internal/auth"
	"kontroler-server/internal/config"
	"kontroler-server/internal/db"
	kclient "kontroler-server/internal/kClient"
	"kontroler-server/internal/logs"
	internalRest "kontroler-server/internal/rest"
	"kontroler-server/internal/ws"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/websocket/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	var configPath string

	flag.StringVar(&configPath, "configpath", "", "Path to configuration file")
	flag.Parse()

	serverConfig, err := config.ParseConfig(configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse config")
	}

	// Get kubernetes config
	var config *rest.Config
	var kubeErr error
	if serverConfig.KubeConfigPath != "" {
		config, kubeErr = clientcmd.BuildConfigFromFlags("", serverConfig.KubeConfigPath)
	} else {
		config, kubeErr = rest.InClusterConfig()
	}

	if kubeErr != nil {
		log.Fatal().Err(kubeErr).Msg("failed to create kubernetes config")
	}

	auditLogs, _ := os.LookupEnv("AUDIT_LOGS")

	corsUiAddress, exists := os.LookupEnv("CORS_UI_ADDRESS")
	if !exists {
		panic("missing CORS_UI_ADDRESS")
	}

	jwtKey, exists := os.LookupEnv("JWT_KEY")
	if !exists {
		panic("missing JWT_KEY")
	}

	var dbDAGManager db.DbManager
	var authManager auth.AuthManager

	ctx := context.Background()

	switch os.Getenv("DB_TYPE") {
	case "postgresql":
		pgConfig, err := db.ConfigurePostgres()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create postgres config")
		}

		pool, err := pgxpool.NewWithConfig(ctx, pgConfig)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create postgres pool")
		}

		defer pool.Close()

		dbDAGManager, err = db.NewPostgresManager(ctx, pool)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create postgres DAG manager")
		}

		authManager, err = auth.NewAuthPostgresManager(ctx, pool, jwtKey)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create postgres auth manager")
		}

	case "sqlite":

		config, err := db.ConfigureSqlite()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create sqlite config")
		}

		dbDAGManager, err = db.NewSQLiteReadOnlyManager(ctx, config)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create sqlite DAG manager")
		}

		configAuth, err := auth.ConfigureAuthSqlite()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create sqlite config")
		}

		dbSqlite, err := sql.Open("sqlite", configAuth.DBPath)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to open SQLite database")
		}

		authManager, err = auth.NewAuthSQLiteManager(ctx, dbSqlite, configAuth, jwtKey)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create auth manager")
		}

	default:
		log.Fatal().Msg("unsupported DAG manager provided, 'postgresql' or 'sqlite'")
	}

	defer dbDAGManager.Close()

	if err := authManager.InitialiseDatabase(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to initialise the database for auth management")
	}

	kubClient, clientset, err := kclient.NewClients(config)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create a kubernetes client")
	}

	logFetcher, err := logs.NewLogFetcher(os.Getenv("S3_BUCKETNAME"))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create a log fetcher")
	}

	logStreamer := ws.NewWebSocketLogStream(dbDAGManager, clientset)

	app := internalRest.NewFiberHttpServer(dbDAGManager, kubClient, authManager, corsUiAddress, strings.ToLower(auditLogs) == "true", logFetcher)

	// Apply authentication middleware BEFORE WebSocket upgrade
	app.Use("/ws/logs", ws.Auth(authManager))

	app.Use("/ws/logs", limiter.New(limiter.Config{
		Max:        30,
		Expiration: 10 * time.Minute,
	}))

	// Secure WebSocket route
	app.Get("/ws/logs", websocket.New(logStreamer.StreamLogs))

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Info().Msg("Prometheus metrics endpoint is available at :2112/metrics")
		if err := http.ListenAndServe(":2112", nil); err != nil {
			log.Fatal().Err(err).Msg("Error starting metrics server")
		}
	}()

	// Create a channel to listen for OS signals
	quit := make(chan os.Signal, 1)

	// syscall.SIGTERM is for kubernetes
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	var tlsConfig *tls.Config = nil
	if os.Getenv("MTLS") == "true" {
		tlsConfig, err = internalRest.CreateTLSConfig()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create rest tls configuration")
		}
	}

	// Run Fiber server in a separate goroutine
	go func() {
		log.Info().Int("port", 8080).Msg("listening on port")

		if tlsConfig == nil {
			log.Info().Msg("Starting server with http")
			if err := app.Listen(":8080"); err != nil {
				log.Fatal().Err(err).Msg("Error starting server")
			}
		} else {
			log.Info().Msg("Starting server with tls")
			ln, err := net.Listen("tcp", ":8080")
			if err != nil {
				log.Fatal().Err(err).Msg("Error starting server listener")
			}

			ln = tls.NewListener(ln, tlsConfig)

			if err := app.Listener(ln); err != nil {
				log.Fatal().Err(err).Msg("Error starting server with mtls")
			}
		}
	}()

	// Wait for OS signal to gracefully shutdown the server
	<-quit
	log.Info().Msg("Shutting down server...")

	// Set a deadline for shutdown
	ctx, cancelShutdown := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelShutdown()

	// Shutdown the server gracefully
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Error().Err(err).Msg("Error shutting down server")
	}
	log.Info().Msg("Server gracefully stopped")
}
