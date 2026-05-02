package auth

import (
	"fmt"
	"os"
)

// AuthSQLiteConfig holds the configurable SQLite settings
type AuthSQLiteConfig struct {
	DBPath      string
	JournalMode string // e.g., "WAL"
	Synchronous string // e.g., "NORMAL" or "FULL"
	CacheSize   int    // e.g., -2000 (for KB, negative to use memory size in KB)
	TempStore   string // e.g., "MEMORY"
}

func ConfigureAuthSqlite() (*AuthSQLiteConfig, error) {
	config := &AuthSQLiteConfig{}

	config.DBPath = os.Getenv("AUTH_SQLITE_PATH")
	if config.DBPath == "" {
		return nil, fmt.Errorf("missing SQLITE_PATH")
	}

	config.JournalMode = os.Getenv("SQLITE_JOURNAL_MODE")
	if config.JournalMode == "" {
		config.JournalMode = "WAL"
	}

	config.Synchronous = os.Getenv("SQLITE_SYNCHRONOUS")
	if config.Synchronous == "" {
		config.Synchronous = "NORMAL"
	}

	cacheSize := os.Getenv("SQLITE_CACHE_SIZE")
	if cacheSize == "" {
		config.CacheSize = -2000
	}

	config.TempStore = os.Getenv("SQLITE_TEMP_STORE")
	if config.TempStore == "" {
		config.TempStore = "MEMORY"
	}

	return config, nil
}
