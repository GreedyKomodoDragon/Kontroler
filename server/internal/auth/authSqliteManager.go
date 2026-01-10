package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type authSqliteManager struct {
	db        *sql.DB
	secretKey []byte
}

func NewAuthSQLiteManager(ctx context.Context, db *sql.DB, config *AuthSQLiteConfig, secretKey string) (AuthManager, error) {
	// Apply the configurable settings if provided
	if config.JournalMode != "" {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA journal_mode=%s;", config.JournalMode)); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set journal mode: %w", err)
		}
	}

	if config.Synchronous != "" {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA synchronous=%s;", config.Synchronous)); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
		}
	}

	if config.CacheSize != 0 {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA cache_size=%d;", config.CacheSize)); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set cache size: %w", err)
		}
	}

	if config.TempStore != "" {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA temp_store=%s;", config.TempStore)); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set temp store using value %s: %w", config.TempStore, err)
		}
	}

	// Check the connection to ensure the database is accessible.
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to SQLite database: %w", err)
	}

	return &authSqliteManager{
		db:        db,
		secretKey: []byte(secretKey),
	}, nil

}

// InitialiseDatabase implements AuthManager.
func (a *authSqliteManager) InitialiseDatabase(ctx context.Context) error {
	// Create tables for accounts and tokens
	initSQL := `
		CREATE TABLE IF NOT EXISTS accounts (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'viewer',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS tokens (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			account_id TEXT REFERENCES accounts(id) ON DELETE CASCADE,
			token TEXT UNIQUE NOT NULL,
			expires_at TEXT NOT NULL,
			revoked BOOLEAN NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_expires_at ON tokens(expires_at);
	`

	if _, err := a.db.ExecContext(ctx, initSQL); err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}

	// Check if the default account already exists
	var count int
	checkAccountSQL := `SELECT COUNT(*) FROM accounts WHERE username = ?;`
	if err := a.db.QueryRowContext(ctx, checkAccountSQL, "admin").Scan(&count); err != nil {
		return fmt.Errorf("failed to check for default account: %v", err)
	}

	// If the account does not exist, create the default account
	if count == 0 {
		createDefaultAccountSQL := `
			INSERT INTO accounts (username, password_hash, role) 
			VALUES (?, ?, ?);
		`

		adminPassword := os.Getenv("DEFAULT_ADMIN_PASSWORD")
		if adminPassword == "" {
			adminPassword = "adminpassword" // fallback for development
		}

		hashedPassword, err := hashPassword(adminPassword)
		if err != nil {
			return err
		}

		if _, err := a.db.ExecContext(ctx, createDefaultAccountSQL, "admin", hashedPassword, "admin"); err != nil {
			return fmt.Errorf("failed to create default account: %v", err)
		}
	}

	return nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (a *authSqliteManager) ChangePassword(ctx context.Context, username string, changeCredentials ChangeCredentials) error {
	var storedPasswordHash string

	// Query to get the stored password hash for the username
	query := `
		SELECT password_hash
		FROM accounts
		WHERE username = ?`
	if err := a.db.QueryRowContext(ctx, query, username).Scan(&storedPasswordHash); err != nil {
		if err == sql.ErrNoRows {
			return errors.New("user not found")
		}
		return fmt.Errorf("failed to query for user: %v", err)
	}

	// Check if the old password matches the stored password hash
	err := bcrypt.CompareHashAndPassword([]byte(storedPasswordHash), []byte(changeCredentials.OldPassword))
	if err != nil {
		return errors.New("incorrect old password")
	}

	// Hash the new password using bcrypt
	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(changeCredentials.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %v", err)
	}

	// Update the password hash and updated_at timestamp in the database
	updateQuery := `
		UPDATE accounts
		SET password_hash = ?, updated_at = ?
		WHERE username = ?`
	_, err = a.db.ExecContext(ctx, updateQuery, newPasswordHash, time.Now(), username)
	if err != nil {
		return fmt.Errorf("failed to update password: %v", err)
	}

	return nil
}

func (a *authSqliteManager) CreateAccount(ctx context.Context, credentials *CreateAccountReq) error {
	createAccountSQL := `
		INSERT INTO accounts (username, password_hash, role) 
		VALUES (?, ?, ?);
	`

	hashedPassword, err := hashPassword(credentials.Password)
	if err != nil {
		return err
	}

	if _, err := a.db.ExecContext(ctx, createAccountSQL, credentials.Username, hashedPassword, credentials.Role); err != nil {
		return fmt.Errorf("failed to create account: %v", err)
	}

	return nil
}

func (a *authSqliteManager) DeleteUser(ctx context.Context, user string) error {
	if user == "admin" {
		return fmt.Errorf("cannot delete the admin account")
	}

	if _, err := a.db.ExecContext(ctx, `
	DELETE FROM accounts 
	WHERE username = ?;
	`, user); err != nil {
		return err
	}

	return nil
}

func (a *authSqliteManager) GetUserPageCount(ctx context.Context, limit int) (int, error) {
	var pageCount int

	if err := a.db.QueryRowContext(ctx, `
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

func (a *authSqliteManager) GetUsers(ctx context.Context, limit int, offset int) ([]*User, error) {
	rows, err := a.db.QueryContext(ctx, `
	SELECT username, role
	FROM accounts
	ORDER BY created_at DESC
	LIMIT ? OFFSET ?;
	`, limit, offset)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	users := []*User{}
	for rows.Next() {
		user := User{}
		if err := rows.Scan(&user.Username, &user.Role); err != nil {
			return nil, err
		}

		users = append(users, &user)
	}

	return users, nil
}

func (a *authSqliteManager) IsValidLogin(ctx context.Context, tokenString string) (string, string, error) {
	// Parse and verify the JWT token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Ensure that the token method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.secretKey, nil
	})

	if err != nil || !token.Valid {
		return "", "", fmt.Errorf("invalid token")
	}

	// Extract claims from the token
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", fmt.Errorf("invalid token claims")
	}

	// Get the role from the token claims
	role, ok := claims["role"].(string)
	if !ok {
		return "", "", fmt.Errorf("role not found in token")
	}

	// Query the token details from the database
	var revoked bool
	var accountId string // UUIDs stored as strings in SQLite
	if err := a.db.QueryRowContext(ctx, `
		SELECT revoked, account_id
		FROM tokens 
		WHERE token = ? AND expires_at > CURRENT_TIMESTAMP;
	`, tokenString).Scan(&revoked, &accountId); err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf("invalid or expired token")
		}
		return "", "", fmt.Errorf("failed to query token: %v", err)
	}

	// Check if the token has been revoked
	if revoked {
		return "", "", fmt.Errorf("token has been revoked")
	}

	// Query the username based on account ID
	var username string
	if err := a.db.QueryRowContext(ctx, `
		SELECT username
		FROM accounts 
		WHERE id = ?;
	`, accountId).Scan(&username); err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf("could not find username")
		}
		return "", "", fmt.Errorf("failed to query for username: %v", err)
	}

	return username, role, nil
}

func (a *authSqliteManager) Login(ctx context.Context, credentials *Credentials) (string, string, error) {
	var accountId string
	var storedPasswordHash string
	var role string

	if err := a.db.QueryRowContext(ctx, `
		SELECT id, password_hash, role
		FROM accounts 
		WHERE username = ?;
	`, credentials.Username).Scan(&accountId, &storedPasswordHash, &role); err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf("invalid credentials")
		}
		return "", "", fmt.Errorf("failed to query user: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(storedPasswordHash), []byte(credentials.Password)); err != nil {
		return "", "", fmt.Errorf("invalid credentials")
	}

	now := time.Now()
	expires := now.Add(24 * time.Hour)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"account_id": accountId,
		"exp":        expires.Unix(),
		"iat":        now.Unix(),
		"jti":        uuid.New().String(),
		"role":       role,
	})

	signedToken, err := token.SignedString(a.secretKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign token: %v", err)
	}

	_, err = a.db.ExecContext(ctx, `
		INSERT INTO tokens (account_id, token, expires_at)
		VALUES (?, ?, ?);
	`, accountId, signedToken, expires)
	if err != nil {
		return "", "", fmt.Errorf("failed to store token: %v", err)
	}

	return signedToken, role, nil
}

func (a *authSqliteManager) LoginWithRateLimit(ctx context.Context, credentials *Credentials, rateLimiter *RateLimiter) (string, string, error) {
	// Check if the IP or username is blocked (using username as identifier)
	if rateLimiter.IsBlocked(credentials.Username) {
		return "", "", fmt.Errorf("too many failed login attempts. please try again later")
	}

	// Attempt login
	token, role, err := a.Login(ctx, credentials)
	if err != nil {
		// Record failed attempt
		if rateLimiter.RecordFailure(credentials.Username) {
			return "", "", fmt.Errorf("too many failed login attempts. account temporarily blocked")
		}
		return "", "", err
	}

	// Record successful login (clears the rate limit counter)
	rateLimiter.RecordSuccess(credentials.Username)

	return token, role, nil
}

func (a *authSqliteManager) RevokeToken(ctx context.Context, tokenString string) error {
	if _, err := a.db.ExecContext(ctx, `
		UPDATE tokens 
		SET revoked = TRUE 
		WHERE token = ?;
	`, tokenString); err != nil {
		return fmt.Errorf("failed to revoke token: %v", err)
	}

	return nil
}
