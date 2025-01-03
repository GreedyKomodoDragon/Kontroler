package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"kontroler-server/pkg/auth"
	"kontroler-server/pkg/db"
	kclient "kontroler-server/pkg/kClient"
	"kontroler-server/pkg/logs"
	"kontroler-server/pkg/rest"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

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

	kubClient, err := kclient.NewClient()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create a kubernetes client")
	}

	logFetcher, err := logs.NewLogFetcher(os.Getenv("S3_BUCKETNAME"))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create a log fetcher")
	}

	app := rest.NewFiberHttpServer(dbDAGManager, kubClient, authManager, corsUiAddress, strings.ToLower(auditLogs) == "true", logFetcher)

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
		tlsConfig, err = rest.CreateTLSConfig()
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
