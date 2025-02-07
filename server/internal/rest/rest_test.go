package rest_test

import (
	"kontroler-server/internal/auth"
	"kontroler-server/internal/rest"
	"testing"
)

func TestValidateCredentials(t *testing.T) {
	tests := []struct {
		name         string
		credentials  auth.CreateAccountReq
		expectError  bool
		errorMessage string
	}{
		// Valid cases
		{
			name: "Valid username and password",
			credentials: auth.CreateAccountReq{
				Username: "ValidUser1",
				Password: "Password123",
				Role:     "viewer"},
			expectError: false,
		},
		{
			name: "Valid length",
			credentials: auth.CreateAccountReq{
				Username: "AReallyLongUsernameThatLessThaCharsLongWhichIsValid12345",
				Password: "ValidPassword123",
				Role:     "viewer"},
			expectError: false,
		},

		// Invalid username cases
		{
			name:         "Username too short",
			credentials:  auth.CreateAccountReq{Username: "ab", Password: "ValidPassword123", Role: "viewer"},
			expectError:  true,
			errorMessage: "username must be between 3 and 100 characters long",
		},
		{
			name: "Username too long",
			credentials: auth.CreateAccountReq{
				Username: "ThisIsAVeryLongUsernameThatExceedsOneHundredCharactersAndIsUsedToTestTheUsernameValidationHskajdklsajdj",
				Password: "ValidPassword123",
				Role:     "viewer",
			},
			expectError:  true,
			errorMessage: "username must be between 3 and 100 characters long",
		},
		{
			name: "Username starts with a number",
			credentials: auth.CreateAccountReq{
				Username: "1InvalidUser",
				Password: "Password123",
				Role:     "viewer"},
			expectError:  true,
			errorMessage: "username must start with a letter",
		},
		{
			name: "Username contains invalid characters",
			credentials: auth.CreateAccountReq{
				Username: "Invalid@User",
				Password: "Password123",
				Role:     "viewer"},
			expectError:  true,
			errorMessage: "username must use only letter or number characters",
		},
		{
			name: "Password too short",
			credentials: auth.CreateAccountReq{
				Username: "ValidUser",
				Password: "short",
				Role:     "viewer"},
			expectError:  true,
			errorMessage: "password must be at least 8 characters long",
		},
		{
			name: "Password without letters",
			credentials: auth.CreateAccountReq{
				Username: "ValidUser",
				Password: "12345678",
				Role:     "viewer"},
			expectError:  true,
			errorMessage: "password must contain at least one letter",
		},
		{
			name: "Password contains invalid characters",
			credentials: auth.CreateAccountReq{
				Username: "ValidUser",
				Password: "Invalid@Password",
				Role:     "viewer"},
			expectError:  true,
			errorMessage: "password must use only letter or number characters",
		},
		{
			name: "Invalid role",
			credentials: auth.CreateAccountReq{
				Username: "ValidUser",
				Password: "ValidPassword123",
				Role:     "invalid_role"},
			expectError:  true,
			errorMessage: "role must be either admin, editor, or viewer",
		},
		{
			name: "Empty role",
			credentials: auth.CreateAccountReq{
				Username: "ValidUser",
				Password: "ValidPassword123",
				Role:     ""},
			expectError:  true,
			errorMessage: "role must be either admin, editor, or viewer",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := rest.ValidateCredentials(test.credentials)
			if (err != nil) != test.expectError {
				t.Fatalf("Expected error: %v, got: %v for %v", test.expectError, err != nil, test.credentials)
			}
			if err != nil && err.Error() != test.errorMessage {
				t.Fatalf("Expected error message: %s, got: %s for %v", test.errorMessage, err.Error(), test.credentials)
			}
		})
	}
}
