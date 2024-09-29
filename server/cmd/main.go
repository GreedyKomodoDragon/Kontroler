package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"kontroler-server/pkg/auth"
	"kontroler-server/pkg/db"
	kclient "kontroler-server/pkg/kClient"
	"kontroler-server/pkg/rest"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	dbName, exists := os.LookupEnv("DB_NAME")
	if !exists {
		panic("missing DB_NAME")
	}

	dbUser, exists := os.LookupEnv("DB_USER")
	if !exists {
		panic("missing DB_USER")
	}

	jwtKey, exists := os.LookupEnv("JWT_KEY")
	if !exists {
		panic("missing JWT_KEY")
	}

	dbPassword, exists := os.LookupEnv("DB_PASSWORD")
	if !exists {
		panic("missing DB_PASSWORD")
	}

	auditLogs, _ := os.LookupEnv("AUDIT_LOGS")

	pgEndpoint, exists := os.LookupEnv("DB_ENDPOINT")
	if !exists {
		panic("missing DB_ENDPOINT")
	}

	corsUiAddress, exists := os.LookupEnv("CORS_UI_ADDRESS")
	if !exists {
		panic("missing CORS_UI_ADDRESS")
	}

	pgConfig, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://%s:%s@%s/%s", dbUser, dbPassword, pgEndpoint, dbName))
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, pgConfig)
	if err != nil {
		panic(err)
	}

	dbManager, err := db.NewPostgresManager(ctx, pool)
	if err != nil {
		panic(err)
	}

	authManager, err := auth.NewAuthManager(ctx, pool, jwtKey)
	if err != nil {
		panic(err)
	}

	if err := authManager.InitialiseDatabase(ctx); err != nil {
		panic(err)
	}

	kubClient, err := kclient.NewClient()
	if err != nil {
		panic(err)
	}

	app := rest.NewFiberHttpServer(dbManager, kubClient, authManager, corsUiAddress, strings.ToLower(auditLogs) == "true")

	// Create a channel to listen for OS signals
	quit := make(chan os.Signal, 1)

	// syscall.SIGTERM is for kubernetes
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	var tlsConfig *tls.Config = nil
	if os.Getenv("MTLS") == "true" {
		tlsConfig, err = rest.CreateTLSConfig()
		if err != nil {
			panic(err)
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
