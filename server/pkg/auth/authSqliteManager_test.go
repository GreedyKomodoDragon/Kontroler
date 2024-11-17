package auth_test

import (
	"context"
	"database/sql"
	"fmt"
	"kontroler-server/pkg/auth"
	"math/rand"
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func Test_SQLite_AuthManager(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))

	dbSqlite, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Error().Err(err).Msg("failed to open SQLite database")
	}

	// Initialize authManager
	authManager, err := auth.NewAuthSQLiteManager(context.Background(), dbSqlite, &auth.AuthSQLiteConfig{}, "randomKey")
	require.NoError(t, err)

	test_Setup_AuthManager(t, authManager)
}

func Test_Sqlite_CreateAccount(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))

	dbSqlite, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Error().Err(err).Msg("failed to open SQLite database")
	}

	// Initialize authManager
	authManager, err := auth.NewAuthSQLiteManager(context.Background(), dbSqlite, &auth.AuthSQLiteConfig{}, "randomKey")
	require.NoError(t, err)

	credentials := &auth.Credentials{
		Username: "testuser",
		Password: "testpassword",
	}

	test_CreateAccount_Valid(t, authManager, credentials)

	// Verify the account was created
	var passwordHash string
	err = dbSqlite.QueryRowContext(context.Background(),
		`SELECT password_hash
		FROM accounts
		WHERE username = ?`,
		credentials.Username).Scan(&passwordHash)

	require.NoError(t, err)
	require.NotEmpty(t, passwordHash)

	credentials = &auth.Credentials{
		Username: "randomUser",
		Password: "testpassword",
	}

	test_CreateAccount_UsernameAlreadyExists(t, authManager, credentials)

	var count int
	err = dbSqlite.QueryRowContext(context.Background(), `
	SELECT COUNT(*)
	FROM accounts
	WHERE username = ?;`, credentials.Username).Scan(&count)

	require.NoError(t, err)
	assert.Equal(t, 1, count, "Expected exactly one account with the username")
}

func Test_SQLite_Login(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))

	dbSqlite, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Error().Err(err).Msg("failed to open SQLite database")
	}

	authManager, err := auth.NewAuthSQLiteManager(context.Background(), dbSqlite, &auth.AuthSQLiteConfig{}, "randomKey")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	credentials := &auth.Credentials{
		Username: "testuser",
		Password: "testpassword",
	}

	test_valid_login(t, authManager, credentials)

	credentials = &auth.Credentials{
		Username: "ailsjdilasd",
		Password: "laksjdhlas",
	}

	test_invalid_login(t, authManager, credentials)
}

func Test_SQLite_IsValidLogin(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))

	dbSqlite, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Error().Err(err).Msg("failed to open SQLite database")
	}

	authManager, err := auth.NewAuthSQLiteManager(context.Background(), dbSqlite, &auth.AuthSQLiteConfig{}, "randomKey")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	credentials := &auth.Credentials{
		Username: "testuser",
		Password: "testpassword",
	}

	test_is_valid_login(t, authManager, credentials)
}

func Test_SQLite_RevokeToken(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))

	dbSqlite, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Error().Err(err).Msg("failed to open SQLite database")
	}

	authManager, err := auth.NewAuthSQLiteManager(context.Background(), dbSqlite, &auth.AuthSQLiteConfig{}, "randomKey")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	credentials := &auth.Credentials{
		Username: "testuser",
		Password: "testpassword",
	}

	test_revoke_token(t, authManager, credentials)
}

func Test_Sqlite_TokenExpiration(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))

	dbSqlite, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Error().Err(err).Msg("failed to open SQLite database")
	}

	authManager, err := auth.NewAuthSQLiteManager(context.Background(), dbSqlite, &auth.AuthSQLiteConfig{}, "randomKey")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	credentials := &auth.Credentials{
		Username: "testuser",
		Password: "testpassword",
	}

	// Create the account and login to get a token
	err = authManager.CreateAccount(context.Background(), credentials)
	require.NoError(t, err)

	token, err := authManager.Login(context.Background(), credentials)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Simulate token expiration
	_, err = dbSqlite.ExecContext(context.Background(), `
		UPDATE tokens
		SET expires_at = DATETIME(CURRENT_TIMESTAMP, '-1 day')
		WHERE token = ?;
	`, token)
	require.NoError(t, err)

	// Validate the expired token
	id, err := authManager.IsValidLogin(context.Background(), token)
	assert.Error(t, err, "expected token to be invalid due to expiration")
	require.Empty(t, id)
}

func Test_SQLite_ChangePassword(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))

	dbSqlite, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Error().Err(err).Msg("failed to open SQLite database")
	}

	authManager, err := auth.NewAuthSQLiteManager(context.Background(), dbSqlite, &auth.AuthSQLiteConfig{}, "randomKey")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	credentials := &auth.Credentials{
		Username: "testuser",
		Password: "testpassword",
	}

	test_change_password(t, authManager, credentials)
}