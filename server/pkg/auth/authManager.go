package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type User struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

type AuthManager interface {
	InitialiseDatabase(ctx context.Context) error
	CreateAccount(ctx context.Context, crednetials *Credentials) error
	Login(ctx context.Context, crednetials *Credentials) (string, error)
	IsValidLogin(ctx context.Context, token string) (string, error)
	RevokeToken(ctx context.Context, tokenString string) error
	GetUsers(ctx context.Context, limit, offset int) ([]*User, error)
	GetUserPageCount(ctx context.Context, limit int) (int, error)
	DeleteUser(ctx context.Context, user string) error
}

type authManager struct {
	pool      *pgxpool.Pool
	secretKey []byte
}

func NewAuthManager(ctx context.Context, pool *pgxpool.Pool, secretKey string) (AuthManager, error) {
	// Using pool allows for different database instance to the one that handles all the dags
	// TODO: Add option to use a different database

	return &authManager{
		pool:      pool,
		secretKey: []byte(secretKey),
	}, nil

}

func (a *authManager) InitialiseDatabase(ctx context.Context) error {
	initSQL := `
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
		CREATE EXTENSION IF NOT EXISTS pgcrypto;

		CREATE TABLE IF NOT EXISTS accounts (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			username VARCHAR(255) UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS tokens (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			account_id UUID REFERENCES accounts(id) ON DELETE CASCADE,
			token TEXT UNIQUE NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			revoked BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_expires_at ON tokens(expires_at);
	`

	// Execute the SQL to initialize the database
	if _, err := a.pool.Exec(ctx, initSQL); err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}

	// Check if the default account already exists
	var count int
	checkAccountSQL := `
		SELECT COUNT(*) FROM accounts WHERE username = $1;
	`
	if err := a.pool.QueryRow(ctx, checkAccountSQL, "admin").Scan(&count); err != nil {
		return fmt.Errorf("failed to check for default account: %v", err)
	}

	if count == 0 {
		createDefaultAccountSQL := `
			INSERT INTO accounts (username, password_hash) 
			VALUES ($1, crypt($2, gen_salt('bf')));
		`

		// TODO: move out adminpassword to a ENV
		if _, err := a.pool.Exec(ctx, createDefaultAccountSQL, "admin", "adminpassword"); err != nil {
			return fmt.Errorf("failed to create default account: %v", err)
		}
	}

	return nil
}

func (a *authManager) CreateAccount(ctx context.Context, credentials *Credentials) error {
	// Insert the new user into the accounts table with hashed password
	if _, err := a.pool.Exec(ctx, `
		INSERT INTO accounts (username, password_hash) 
		VALUES ($1, crypt($2, gen_salt('bf')))
	`, credentials.Username, credentials.Password); err != nil {
		return fmt.Errorf("failed to create account: %v", err)
	}

	return nil
}

func (a *authManager) Login(ctx context.Context, credentials *Credentials) (string, error) {
	var accountId uuid.UUID

	if err := a.pool.QueryRow(ctx, `
		SELECT id 
		FROM accounts 
		WHERE username = $1 AND password_hash = crypt($2, password_hash)
	`, credentials.Username, credentials.Password).Scan(&accountId); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("invalid credentials")
		}
		return "", fmt.Errorf("failed to query user: %v", err)
	}

	now := time.Now()
	expires := now.Add(24 * time.Hour)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"account_id": accountId.String(),
		"exp":        expires.Unix(),
		"iat":        now.Unix(),
		"jti":        uuid.New().String(),
	})

	signedToken, err := token.SignedString(a.secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %v", err)
	}

	if _, err = a.pool.Exec(ctx, `
		INSERT INTO tokens (account_id, token, expires_at)
		VALUES ($1, $2, $3)
	`, accountId, signedToken, expires); err != nil {
		return "", fmt.Errorf("failed to store token: %v", err)
	}

	return signedToken, nil
}

func (a *authManager) IsValidLogin(ctx context.Context, tokenString string) (string, error) {
	// Parse and verify the JWT token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Ensure that the token method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.secretKey, nil
	})

	if err != nil || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	var revoked bool
	var accountId uuid.UUID
	if err := a.pool.QueryRow(ctx, `
		SELECT revoked, account_id
		FROM tokens 
		WHERE token = $1 AND expires_at > NOW()
	`, tokenString).Scan(&revoked, &accountId); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("invalid or expired token")
		}
		return "", fmt.Errorf("failed to query token: %v", err)
	}

	if revoked {
		return "", fmt.Errorf("token has been revoked")
	}

	return accountId.String(), nil
}

func (a *authManager) RevokeToken(ctx context.Context, tokenString string) error {
	if _, err := a.pool.Exec(ctx, `
		UPDATE tokens 
		SET revoked = TRUE 
		WHERE token = $1
	`, tokenString); err != nil {
		return fmt.Errorf("failed to revoke token: %v", err)
	}

	return nil
}

func (a *authManager) GetUsers(ctx context.Context, limit, offset int) ([]*User, error) {
	rows, err := a.pool.Query(ctx, `
	SELECT username
	FROM accounts
	ORDER BY created_at DESC
	LIMIT $1 OFFSET $2;
	`, limit, offset)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	users := []*User{}
	for rows.Next() {
		user := User{}
		if err := rows.Scan(&user.Username); err != nil {
			return nil, err
		}

		user.Role = "Admin"

		users = append(users, &user)
	}

	return users, nil
}

func (a *authManager) GetUserPageCount(ctx context.Context, limit int) (int, error) {
	var pageCount int

	if err := a.pool.QueryRow(ctx, `
	SELECT COUNT(*)
	FROM accounts;
	`).Scan(&pageCount); err != nil {
		return 0, err
	}

	pages := pageCount / limit
	if pageCount%limit > 0 {
		pages++
	}

	return pages, nil
}

func (a *authManager) DeleteUser(ctx context.Context, user string) error {
	if user == "admin" {
		return fmt.Errorf("Cannot delete the admin account")
	}

	if _, err := a.pool.Exec(ctx, `
	DELETE FROM accounts 
	WHERE username = $1;
	`, user); err != nil {
		return err
	}

	return nil
}
