package auth_test

import (
	"context"
	"kontroler-server/pkg/auth"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func test_Setup_AuthManager(t *testing.T, authManager auth.AuthManager) {
	t.Run("Setup DB", func(t *testing.T) {
		err := authManager.InitialiseDatabase(context.Background())
		require.NoError(t, err)
	})
}

func test_CreateAccount_Valid(t *testing.T, authManager auth.AuthManager, credentials *auth.Credentials) {
	t.Run("Success Path create account", func(t *testing.T) {
		err := authManager.InitialiseDatabase(context.Background())
		require.NoError(t, err)

		err = authManager.CreateAccount(context.Background(), credentials)
		require.NoError(t, err)
	})
}

func test_CreateAccount_UsernameAlreadyExists(t *testing.T, authManager auth.AuthManager, credentials *auth.Credentials) {
	t.Run("Failed Dup account", func(t *testing.T) {
		// Create the first account
		err := authManager.CreateAccount(context.Background(), credentials)
		require.NoError(t, err)

		// Attempt to create a second account with the same username
		err = authManager.CreateAccount(context.Background(), credentials)

		assert.Error(t, err)
	})

}

func test_valid_login(t *testing.T, authManager auth.AuthManager, credentials *auth.Credentials) {
	t.Run("valid login for account", func(t *testing.T) {
		err := authManager.CreateAccount(context.Background(), credentials)
		require.NoError(t, err)

		token, err := authManager.Login(context.Background(), credentials)
		require.NoError(t, err)
		require.NotEmpty(t, token)
	})
}

func test_invalid_login(t *testing.T, authManager auth.AuthManager, credentials *auth.Credentials) {
	t.Run("invalid login for account", func(t *testing.T) {
		token, err := authManager.Login(context.Background(), credentials)
		require.Error(t, err)
		require.Empty(t, token)
	})
}

func test_is_valid_login(t *testing.T, authManager auth.AuthManager, credentials *auth.Credentials) {
	t.Run("is valid login for account", func(t *testing.T) {
		err := authManager.CreateAccount(context.Background(), credentials)
		require.NoError(t, err)

		token, err := authManager.Login(context.Background(), credentials)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		id, err := authManager.IsValidLogin(context.Background(), token)
		require.NoError(t, err)
		require.NotEmpty(t, id)
	})
}

func test_revoke_token(t *testing.T, authManager auth.AuthManager, credentials *auth.Credentials) {
	t.Run("is valid login and revoke for account", func(t *testing.T) {
		// Create account and login to get token
		err := authManager.CreateAccount(context.Background(), credentials)
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
	})
}

func test_change_password(t *testing.T, authManager auth.AuthManager, credentials *auth.Credentials) {
	t.Run("changing password", func(t *testing.T) {
		err := authManager.CreateAccount(context.Background(), credentials)
		require.NoError(t, err)

		token, err := authManager.Login(context.Background(), credentials)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		id, err := authManager.IsValidLogin(context.Background(), token)
		require.NoError(t, err)
		require.NotEmpty(t, id)

		err = authManager.ChangePassword(context.Background(), credentials.Username, auth.ChangeCredentials{
			OldPassword: credentials.Password,
			Password:    "newPassword",
		})
		require.NoError(t, err)

		// Old credentials
		token, err = authManager.Login(context.Background(), credentials)
		require.Error(t, err)
		require.Empty(t, token)

		// New Creds
		token, err = authManager.Login(context.Background(), &auth.Credentials{
			Username: credentials.Username,
			Password: "newPassword",
		})
		require.NoError(t, err)
		require.NotEmpty(t, token)

		id, err = authManager.IsValidLogin(context.Background(), token)
		require.NoError(t, err)
		require.NotEmpty(t, id)
	})
}
