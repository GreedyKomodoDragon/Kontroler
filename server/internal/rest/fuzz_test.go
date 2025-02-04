package rest_test

import (
	"bytes"
	"encoding/json"
	"kontroler-server/internal/auth"
	"kontroler-server/internal/rest"

	"net/http"
	"testing"
)

const (
	apiEndpoint = "http://localhost:8080/api/v1/auth/create"
)

func FuzzAPIFuzzing(f *testing.F) {
	f.Fuzz(func(t *testing.T, name, password string) {
		// Create a new request with the fuzzed input
		body := auth.CreateAccountReq{
			Username: name,
			Password: password,
			Role:     "viewer",
		}

		bBody, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Error creating body: %v", err)
		}

		req, err := http.NewRequest("POST", apiEndpoint, bytes.NewBuffer(bBody))
		if err != nil {
			t.Fatalf("Error creating request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		cookie := &http.Cookie{
			Name:  "jwt-kontroler",
			Value: "",
		}

		req.AddCookie(cookie)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Error sending request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			if resp.StatusCode == http.StatusBadRequest {
				var errorResponse map[string]string
				if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}

				// If there is an error on this, should be an error on the other side
				if errTwo := rest.ValidateCredentials(body); errTwo != nil {
					if errorResponse["message"] == "" {
						t.Fatalf("Expected '%s' error, got: %s, username: %s, password: %s", errTwo.Error(), errorResponse["message"], body.Username, body.Password)
					}
					return
				}

				return
			}

			t.Fatalf("Failed to create account, %s, %s, %v", body.Username, body.Password, resp.StatusCode)
		}
	})
}
