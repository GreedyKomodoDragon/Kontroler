package auth_test

import (
	"context"
	"fmt"
	"kontroler-server/pkg/auth"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupPostgresContainer(t *testing.T) *pgxpool.Pool {
	ctx := context.Background()

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
		t.Fatalf("failed to start container: %v", err)
	}

	host, err := postgresC.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}

	port, err := postgresC.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("failed to get container port: %v", err)
	}

	databaseURL := fmt.Sprintf("postgres://postgres:password@%s:%s/testdb", host, port.Port())
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	// Check if we can acquire a connection
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("failed to acquire connection: %v", err)
	}
	defer conn.Release()

	return pool
}

func Test_AuthManager(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	// Initialize authManager
	authManager, err := auth.NewAuthManager(context.Background(), pool, "key")

	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)
}

func Test_CreateAccount(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthManager(context.Background(), pool, "key")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	credentials := &auth.Credentials{
		Username: "testuser",
		Password: "testpassword",
	}

	err = authManager.CreateAccount(context.Background(), credentials)
	require.NoError(t, err)

	// Verify the account was created
	var passwordHash string
	err = pool.QueryRow(context.Background(), `SELECT password_hash FROM accounts WHERE username = $1`, credentials.Username).Scan(&passwordHash)

	require.NoError(t, err)
	require.NotEmpty(t, passwordHash)
}

func Test_CreateAccount_UsernameAlreadyExists(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthManager(context.Background(), pool, "key")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	// Define account credentials
	credentials := &auth.Credentials{
		Username: "testuser",
		Password: "testpassword",
	}

	// Create the first account
	err = authManager.CreateAccount(context.Background(), credentials)
	require.NoError(t, err)

	// Attempt to create a second account with the same username
	err = authManager.CreateAccount(context.Background(), credentials)

	// Check that the error indicates a unique constraint violation
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate key value violates unique constraint") // Adjust the message based on your actual error handling

	// Optionally, verify that the original account was still created
	var count int
	err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM accounts WHERE username = $1`, credentials.Username).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Expected exactly one account with the username")
}

func Test_Login(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthManager(context.Background(), pool, "key")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	credentials := &auth.Credentials{
		Username: "testuser",
		Password: "testpassword",
	}

	// First create the account
	err = authManager.CreateAccount(context.Background(), credentials)
	require.NoError(t, err)

	// Now attempt to login
	token, err := authManager.Login(context.Background(), credentials)
	require.NoError(t, err)
	require.NotEmpty(t, token)
}

func Test_IsValidLogin(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthManager(context.Background(), pool, "key")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	credentials := &auth.Credentials{
		Username: "testuser",
		Password: "testpassword",
	}

	// Create account and login to get token
	err = authManager.CreateAccount(context.Background(), credentials)
	require.NoError(t, err)

	token, err := authManager.Login(context.Background(), credentials)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Validate the login
	id, err := authManager.IsValidLogin(context.Background(), token)
	require.NoError(t, err)
	require.NotEmpty(t, id)
}

func Test_RevokeToken(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthManager(context.Background(), pool, "key")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	credentials := &auth.Credentials{
		Username: "testuser",
		Password: "testpassword",
	}

	// Create account and login to get token
	err = authManager.CreateAccount(context.Background(), credentials)
	require.NoError(t, err)

	token, err := authManager.Login(context.Background(), credentials)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Revoke the token
	err = authManager.RevokeToken(context.Background(), token)
	require.NoError(t, err)

	// Check if the token is invalid after revocation
	id, err := authManager.IsValidLogin(context.Background(), token)
	require.Error(t, err)
	require.Empty(t, id)
}

func Test_PasswordHashing(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthManager(context.Background(), pool, "key")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	credentials := &auth.Credentials{
		Username: "testuser",
		Password: "testpassword",
	}

	// Create the account
	err = authManager.CreateAccount(context.Background(), credentials)
	require.NoError(t, err)

	// Fetch the hashed password
	var passwordHash string
	err = pool.QueryRow(context.Background(), `SELECT password_hash FROM accounts WHERE username = $1`, credentials.Username).Scan(&passwordHash)
	require.NoError(t, err)

	// Ensure the password hash is not equal to the plain text password
	assert.NotEqual(t, credentials.Password, passwordHash)
}

func Test_TokenExpiration(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthManager(context.Background(), pool, "key")
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
	_, err = pool.Exec(context.Background(), `
		UPDATE tokens 
		SET expires_at = NOW() - INTERVAL '1 day'
		WHERE token = $1
	`, token)
	require.NoError(t, err)

	// Validate the expired token
	id, err := authManager.IsValidLogin(context.Background(), token)
	assert.Error(t, err, "expected token to be invalid due to expiration")
	require.Empty(t, id)
}

func Test_TokenRevocation(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthManager(context.Background(), pool, "key")
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

	// Revoke the token
	err = authManager.RevokeToken(context.Background(), token)
	require.NoError(t, err)

	// Validate the revoked token
	id, err := authManager.IsValidLogin(context.Background(), token)
	assert.Error(t, err, "expected token to be invalid due to revocation")
	require.Empty(t, id)
}

func Test_InvalidLogin(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthManager(context.Background(), pool, "key")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	// Attempt login with non-existent user
	invalidCredentials := &auth.Credentials{
		Username: "nonexistentuser",
		Password: "wrongpassword",
	}

	token, err := authManager.Login(context.Background(), invalidCredentials)
	assert.Error(t, err, "expected error due to invalid credentials")
	assert.Empty(t, token, "expected no token to be returned")
}

func Test_PasswordValidation(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthManager(context.Background(), pool, "key")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	// Create a user
	credentials := &auth.Credentials{
		Username: "testuser",
		Password: "correctpassword",
	}
	err = authManager.CreateAccount(context.Background(), credentials)
	require.NoError(t, err)

	// Try logging in with an incorrect password
	invalidCredentials := &auth.Credentials{
		Username: "testuser",
		Password: "wrongpassword",
	}
	token, err := authManager.Login(context.Background(), invalidCredentials)
	assert.Error(t, err, "expected error due to incorrect password")
	assert.Empty(t, token, "expected no token to be returned for incorrect password")

	// Try logging in with the correct password
	validCredentials := &auth.Credentials{
		Username: "testuser",
		Password: "correctpassword",
	}
	token, err = authManager.Login(context.Background(), validCredentials)
	require.NoError(t, err, "expected successful login with correct password")
	assert.NotEmpty(t, token, "expected a token to be returned for correct password")
}

func Test_TokenReuseAfterExpiration(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthManager(context.Background(), pool, "key")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	// Create a user and log in to get a token
	credentials := &auth.Credentials{
		Username: "testuser",
		Password: "password",
	}
	err = authManager.CreateAccount(context.Background(), credentials)
	require.NoError(t, err)

	token, err := authManager.Login(context.Background(), credentials)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Simulate token expiration
	_, err = pool.Exec(context.Background(), `
		UPDATE tokens 
		SET expires_at = NOW() - INTERVAL '1 day'
		WHERE token = $1
	`, token)
	require.NoError(t, err)

	// Ensure the expired token cannot be reused
	id, err := authManager.IsValidLogin(context.Background(), token)
	assert.Error(t, err, "expected error due to expired token")
	require.Empty(t, id)

	// Try logging in again to get a new token
	token, err = authManager.Login(context.Background(), credentials)
	require.NoError(t, err)
	assert.NotEmpty(t, token, "expected a new token after login with correct credentials")

	// The new token should be valid
	id, err = authManager.IsValidLogin(context.Background(), token)
	assert.NoError(t, err, "expected the new token to be valid")
	require.NotEmpty(t, id)
}
