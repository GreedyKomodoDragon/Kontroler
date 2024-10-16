package rest_test

import (
	"kontroler-server/pkg/auth"
	"kontroler-server/pkg/rest"
	"testing"
)

func TestValidateCredentials(t *testing.T) {
	tests := []struct {
		name         string
		credentials  auth.Credentials
		expectError  bool
		errorMessage string
	}{
		// Valid cases
		{
			name:        "Valid username and password",
			credentials: auth.Credentials{"ValidUser1", "Password123"},
			expectError: false,
		},
		{
			name:        "Valid length",
			credentials: auth.Credentials{"AReallyLongUsernameThatLessThaCharsLongWhichIsValid12345", "ValidPassword123"},
			expectError: false,
		},

		// Invalid username cases
		{
			name:         "Username too short",
			credentials:  auth.Credentials{"ab", "ValidPassword123"},
			expectError:  true,
			errorMessage: "username must be between 3 and 100 characters long",
		},
		{
			name:         "Username too long",
			credentials:  auth.Credentials{"ThisIsAVeryLongUsernameThatExceedsOneHundredCharactersAndIsUsedToTestTheUsernameValidationHskajdklsajdj", "ValidPassword123"},
			expectError:  true,
			errorMessage: "username must be between 3 and 100 characters long",
		},
		{
			name:         "Username starts with a number",
			credentials:  auth.Credentials{"1InvalidUser", "Password123"},
			expectError:  true,
			errorMessage: "username must start with a letter",
		},
		{
			name:         "Username contains invalid characters",
			credentials:  auth.Credentials{"Invalid@User", "Password123"},
			expectError:  true,
			errorMessage: "username must use only letter or number characters",
		},
		{
			name:         "Password too short",
			credentials:  auth.Credentials{"ValidUser", "short"},
			expectError:  true,
			errorMessage: "password must be at least 8 characters long",
		},
		{
			name:         "Password without letters",
			credentials:  auth.Credentials{"ValidUser", "12345678"},
			expectError:  true,
			errorMessage: "password must contain at least one letter",
		},
		{
			name:         "Password contains invalid characters",
			credentials:  auth.Credentials{"ValidUser", "Invalid@Password"},
			expectError:  true,
			errorMessage: "password must use only letter or number characters",
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
