package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"barclays/database/migrations"
	"barclays/internal/storage"
	_ "github.com/glebarez/go-sqlite"
	"github.com/jmoiron/sqlx"
	migrate "github.com/rubenv/sql-migrate"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { db.Close() })

	mg := migrations.GetMigrationSource()
	migrate.SetTable("migrations")
	if _, err = migrate.Exec(db.DB, "sqlite3", mg, migrate.Up); err != nil {
		t.Fatal(err)
	}

	repo := storage.New(db)
	srv := New(Dependencies{Repository: repo})
	return httptest.NewServer(srv.routes)
}

func TestCreateUser(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	client := ts.Client()

	validBody := `{
		"name": "John Doe",
		"address": {
			"line1": "123 Main Street",
			"town": "London",
			"county": "Greater London",
			"postcode": "SW1A 1AA"
		},
		"phoneNumber": "+447911123456",
		"email": "john@example.com",
		"password": "password123"
	}`

	tests := []struct {
		name           string
		body           string
		expectedStatus int
	}{
		{
			name:           "success - all required fields provided",
			body:           validBody,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "duplicate email",
			body:           validBody,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "missing name",
			body:           `{"address":{"line1":"123 Main Street","town":"London","county":"Greater London","postcode":"SW1A 1AA"},"phoneNumber":"+447911123456","email":"other@example.com","password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing address",
			body:           `{"name":"John Doe","phoneNumber":"+447911123456","email":"other@example.com","password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing address line1",
			body:           `{"name":"John Doe","address":{"town":"London","county":"Greater London","postcode":"SW1A 1AA"},"phoneNumber":"+447911123456","email":"other@example.com","password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing address town",
			body:           `{"name":"John Doe","address":{"line1":"123 Main Street","county":"Greater London","postcode":"SW1A 1AA"},"phoneNumber":"+447911123456","email":"other@example.com","password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing address county",
			body:           `{"name":"John Doe","address":{"line1":"123 Main Street","town":"London","postcode":"SW1A 1AA"},"phoneNumber":"+447911123456","email":"other@example.com","password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing address postcode",
			body:           `{"name":"John Doe","address":{"line1":"123 Main Street","town":"London","county":"Greater London"},"phoneNumber":"+447911123456","email":"other@example.com","password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing phone number",
			body:           `{"name":"John Doe","address":{"line1":"123 Main Street","town":"London","county":"Greater London","postcode":"SW1A 1AA"},"email":"other@example.com","password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing email",
			body:           `{"name":"John Doe","address":{"line1":"123 Main Street","town":"London","county":"Greater London","postcode":"SW1A 1AA"},"phoneNumber":"+447911123456","password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing password",
			body:           `{"name":"John Doe","address":{"line1":"123 Main Street","town":"London","county":"Greater London","postcode":"SW1A 1AA"},"phoneNumber":"+447911123456","email":"other@example.com"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty body",
			body:           ``,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Post(ts.URL+"/v1/users", "application/json", bytes.NewBufferString(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			if tc.expectedStatus == http.StatusCreated {
				var body UserResponseBody
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if !strings.HasPrefix(body.ID, "usr-") {
					t.Errorf("id: expected usr- prefix, got %q", body.ID)
				}
				if body.Name != "John Doe" {
					t.Errorf("name: expected %q, got %q", "John Doe", body.Name)
				}
				if body.Email != "john@example.com" {
					t.Errorf("email: expected %q, got %q", "john@example.com", body.Email)
				}
				if body.PhoneNumber != "+447911123456" {
					t.Errorf("phoneNumber: expected %q, got %q", "+447911123456", body.PhoneNumber)
				}
				if body.Address.Line1 != "123 Main Street" {
					t.Errorf("address.line1: expected %q, got %q", "123 Main Street", body.Address.Line1)
				}
				if body.Address.Town != "London" {
					t.Errorf("address.town: expected %q, got %q", "London", body.Address.Town)
				}
				if body.Address.County != "Greater London" {
					t.Errorf("address.county: expected %q, got %q", "Greater London", body.Address.County)
				}
				if body.Address.Postcode != "SW1A 1AA" {
					t.Errorf("address.postcode: expected %q, got %q", "SW1A 1AA", body.Address.Postcode)
				}
				if body.CreatedTimestamp == "" {
					t.Error("expected non-empty createdTimestamp")
				}
				if body.UpdatedTimestamp == "" {
					t.Error("expected non-empty updatedTimestamp")
				}
			}
		})
	}
}

// createAndLogin creates a user with the given email and returns their ID and JWT token.
func createAndLogin(t *testing.T, ts *httptest.Server, email string) (userID string, token string) {
	t.Helper()
	client := ts.Client()

	body := fmt.Sprintf(`{
		"name": "Test User",
		"address": {"line1": "1 Test Street", "town": "London", "county": "Greater London", "postcode": "SW1A 1AA"},
		"phoneNumber": "+447911123456",
		"email": %q,
		"password": "password123"
	}`, email)

	resp, err := client.Post(ts.URL+"/v1/users", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("createAndLogin: create user got %d", resp.StatusCode)
	}
	var created UserResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	loginBody := fmt.Sprintf(`{"email": %q, "password": "password123"}`, email)
	loginResp, err := client.Post(ts.URL+"/v1/auth/login", "application/json", bytes.NewBufferString(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("createAndLogin: login got %d", loginResp.StatusCode)
	}
	var loginResult LoginResponseBody
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		t.Fatal(err)
	}

	return created.ID, loginResult.Token
}

func TestFetchUser(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	client := ts.Client()

	userID, token := createAndLogin(t, ts, "fetch@example.com")
	_, otherToken := createAndLogin(t, ts, "other@example.com")

	authGet := func(url, bearerToken string) *http.Response {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+bearerToken)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	tests := []struct {
		name           string
		userID         string
		token          string
		expectedStatus int
	}{
		{
			name:           "success - fetch own user",
			userID:         userID,
			token:          token,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "forbidden - fetch another user",
			userID:         userID,
			token:          otherToken,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "not found - non-existent user ID",
			userID:         "usr-doesnotexist",
			token:          token,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "unauthorised - no token",
			userID:         userID,
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := authGet(fmt.Sprintf("%s/v1/users/%s", ts.URL, tc.userID), tc.token)
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			if tc.expectedStatus == http.StatusOK {
				var body UserResponseBody
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.ID != userID {
					t.Errorf("id: expected %q, got %q", userID, body.ID)
				}
				if body.Name != "Test User" {
					t.Errorf("name: expected %q, got %q", "Test User", body.Name)
				}
				if body.Email != "fetch@example.com" {
					t.Errorf("email: expected %q, got %q", "fetch@example.com", body.Email)
				}
				if body.CreatedTimestamp == "" {
					t.Error("expected non-empty createdTimestamp")
				}
			}
		})
	}
}
