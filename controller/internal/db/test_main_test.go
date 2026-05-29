package db

import (
	"context"
	"os"
	"testing"

	"kontroler-controller/internal/utils"
)

// TestPGPool is a shared Postgres pool for the package tests when TestMain is used.
// Tests should use db.TestPGPool instead of starting their own container to avoid repeated
// container startup overhead.
var TestPGPool interface{}

func TestMain(m *testing.M) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		// If we cannot start the container, fail fast
		panic(err)
	}

	// expose pool to tests via TestPGPool. Use empty interface to avoid importing pgxpool in this file.
	TestPGPool = pool

	code := m.Run()

	// close pool
	if p, ok := TestPGPool.(interface{ Close() }); ok {
		p.Close()
	}

	os.Exit(code)
}
