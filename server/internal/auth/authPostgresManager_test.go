package auth_test

import (
	"context"
	"fmt"
	"kontroler-server/internal/auth"
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

func Test_Postgres_AuthManager(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	// Initialize authManager
	authManager, err := auth.NewAuthPostgresManager(context.Background(), pool, "key")
	require.NoError(t, err)

	test_Setup_AuthManager(t, authManager)
}

func Test_Postgres_CreateAccount(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthPostgresManager(context.Background(), pool, "key")
	require.NoError(t, err)

	createAccountReq := &auth.CreateAccountReq{
		Username: "testuser",
		Password: "testpassword",
		Role:     "viewer",
	}

	test_CreateAccount_Valid(t, authManager, createAccountReq)

	// Verify the account was created
	var passwordHash string
	err = pool.QueryRow(context.Background(), `SELECT password_hash FROM accounts WHERE username = $1`, createAccountReq.Username).Scan(&passwordHash)

	require.NoError(t, err)
	require.NotEmpty(t, passwordHash)

	createAccountReq = &auth.CreateAccountReq{
		Username: "randomUser",
		Password: "testpassword",
	}

	test_CreateAccount_UsernameAlreadyExists(t, authManager, createAccountReq)

	var count int
	err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM accounts WHERE username = $1`, createAccountReq.Username).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Expected exactly one account with the username")
}

func Test_Postgres_Login(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthPostgresManager(context.Background(), pool, "key")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	createAccountReq := &auth.CreateAccountReq{
		Username: "testuser",
		Password: "testpassword",
		Role:     "viewer",
	}

	test_valid_login(t, authManager, createAccountReq)

	credentials := &auth.Credentials{
		Username: "ailsjdilasd",
		Password: "laksjdhlas",
	}

	test_invalid_login(t, authManager, credentials)
}

func Test_Postgres_IsValidLogin(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthPostgresManager(context.Background(), pool, "key")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	createAccountReq := &auth.CreateAccountReq{
		Username: "testuser",
		Password: "testpassword",
		Role:     "viewer",
	}

	test_is_valid_login(t, authManager, createAccountReq)
}

func Test_Postgres_RevokeToken(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthPostgresManager(context.Background(), pool, "key")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	createAccountReq := &auth.CreateAccountReq{
		Username: "testuser",
		Password: "testpassword",
		Role:     "viewer",
	}

	test_revoke_token(t, authManager, createAccountReq)
}

func Test_Postgres_TokenExpiration(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthPostgresManager(context.Background(), pool, "key")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	createAccountReq := &auth.CreateAccountReq{
		Username: "testuser",
		Password: "testpassword",
		Role:     "viewer",
	}

	credentials := &auth.Credentials{
		Username: createAccountReq.Username,
		Password: createAccountReq.Password,
	}

	// Create the account and login to get a token
	err = authManager.CreateAccount(context.Background(), createAccountReq)
	require.NoError(t, err)

	token, role, err := authManager.Login(context.Background(), credentials)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.Equal(t, "viewer", role)

	// Simulate token expiration
	_, err = pool.Exec(context.Background(), `
		UPDATE tokens 
		SET expires_at = NOW() - INTERVAL '1 day'
		WHERE token = $1
	`, token)
	require.NoError(t, err)

	// Validate the expired token
	id, _, err := authManager.IsValidLogin(context.Background(), token)
	assert.Error(t, err, "expected token to be invalid due to expiration")
	require.Empty(t, id)
}

func Test_Postgres_ChangePassword(t *testing.T) {
	pool := setupPostgresContainer(t)
	defer pool.Close()

	authManager, err := auth.NewAuthPostgresManager(context.Background(), pool, "key")
	require.NoError(t, err)

	err = authManager.InitialiseDatabase(context.Background())
	require.NoError(t, err)

	createAccountReq := &auth.CreateAccountReq{
		Username: "testuser",
		Password: "testpassword",
		Role:     "viewer",
	}

	test_change_password(t, authManager, createAccountReq)
}
