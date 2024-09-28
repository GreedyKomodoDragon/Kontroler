package utils

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TODO: make this loadable from a configmap
var wordList = []string{
	"apple", "banana", "cherry", "orange", "grape", "pear",
	"dog", "cat", "rabbit", "hamster", "turtle", "goldfish",
	"carrot", "broccoli", "potato", "tomato", "onion", "pepper",
	"chair", "table", "lamp", "sofa", "desk", "bookcase",
}

func GenerateRandomName() string {
	// Initialize an empty name
	var name string

	// Choose a random number of words to combine (between 2 and 3)
	numWords := 3

	// Choose random words and combine them
	for i := 0; i < numWords; i++ {
		// Randomly select a word from the list
		wordIndex := rand.Intn(len(wordList))
		name += wordList[wordIndex]

		// If it's not the last word, add a space
		if i < numWords-1 {
			name += "-"
		}
	}

	return name
}

// SetupPostgresContainer sets up a PostgreSQL container for testing.
// It returns a pgxpool.Pool or an error if the setup fails.
func SetupPostgresContainer(ctx context.Context) (*pgxpool.Pool, error) {
	// Request a PostgreSQL container
	req := testcontainers.ContainerRequest{
		Image:        "postgres:13",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_PASSWORD": "password",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}
	postgresC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %v", err)
	}

	host, err := postgresC.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %v", err)
	}

	port, err := postgresC.MappedPort(ctx, "5432")
	if err != nil {
		return nil, fmt.Errorf("failed to get container port: %v", err)
	}

	databaseURL := fmt.Sprintf("postgres://postgres:password@%s:%s/testdb", host, port.Port())
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %v", err)
	}

	// Check if we can acquire a connection
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire connection: %v", err)
	}
	defer conn.Release() // Release the connection after checking

	return pool, nil
}
