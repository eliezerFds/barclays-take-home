package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateAccount(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	client := ts.Client()

	_, token := createAndLogin(t, ts, "account-create@example.com")

	authPost := func(url, body, bearerToken string) *http.Response {
		req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if bearerToken != "" {
			req.Header.Set("Authorization", "Bearer "+bearerToken)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	tests := []struct {
		name           string
		body           string
		token          string
		expectedStatus int
	}{
		{
			name:           "success - all required fields provided",
			body:           `{"name":"Personal Account","accountType":"personal"}`,
			token:          token,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "missing name",
			body:           `{"accountType":"personal"}`,
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing accountType",
			body:           `{"name":"Personal Account"}`,
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid accountType",
			body:           `{"name":"Personal Account","accountType":"business"}`,
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty body",
			body:           ``,
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unauthorised - no token",
			body:           `{"name":"Personal Account","accountType":"personal"}`,
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := authPost(ts.URL+"/v1/accounts", tc.body, tc.token)
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			if tc.expectedStatus == http.StatusCreated {
				var body AccountResponseBody
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if !strings.HasPrefix(body.AccountNumber, "01") || len(body.AccountNumber) != 8 {
					t.Errorf("accountNumber: expected format 01xxxxxx, got %q", body.AccountNumber)
				}
				if body.SortCode != "10-10-10" {
					t.Errorf("sortCode: expected %q, got %q", "10-10-10", body.SortCode)
				}
				if body.Name != "Personal Account" {
					t.Errorf("name: expected %q, got %q", "Personal Account", body.Name)
				}
				if body.AccountType != "personal" {
					t.Errorf("accountType: expected %q, got %q", "personal", body.AccountType)
				}
				if body.Balance != 0.00 {
					t.Errorf("balance: expected 0.00, got %v", body.Balance)
				}
				if body.Currency != "GBP" {
					t.Errorf("currency: expected %q, got %q", "GBP", body.Currency)
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

// createAccount creates a personal account for the given token and returns its account number.
func createAccount(t *testing.T, ts *httptest.Server, token string) string {
	t.Helper()
	client := ts.Client()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/accounts", bytes.NewBufferString(`{"name":"Personal Account","accountType":"personal"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("createAccount: got %d", resp.StatusCode)
	}
	var body AccountResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body.AccountNumber
}

func TestFetchAccount(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	client := ts.Client()

	_, token := createAndLogin(t, ts, "fetch-account@example.com")
	_, otherToken := createAndLogin(t, ts, "fetch-account-other@example.com")
	accountNumber := createAccount(t, ts, token)

	authGet := func(url, bearerToken string) *http.Response {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		if bearerToken != "" {
			req.Header.Set("Authorization", "Bearer "+bearerToken)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	tests := []struct {
		name           string
		accountNumber  string
		token          string
		expectedStatus int
	}{
		{
			name:           "success - fetch own account",
			accountNumber:  accountNumber,
			token:          token,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "forbidden - fetch another user's account",
			accountNumber:  accountNumber,
			token:          otherToken,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "not found - non-existent account number",
			accountNumber:  "01999999",
			token:          token,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "unauthorised - no token",
			accountNumber:  accountNumber,
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := authGet(fmt.Sprintf("%s/v1/accounts/%s", ts.URL, tc.accountNumber), tc.token)
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			if tc.expectedStatus == http.StatusOK {
				var body AccountResponseBody
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.AccountNumber != accountNumber {
					t.Errorf("accountNumber: expected %q, got %q", accountNumber, body.AccountNumber)
				}
				if body.Name != "Personal Account" {
					t.Errorf("name: expected %q, got %q", "Personal Account", body.Name)
				}
				if body.SortCode != "10-10-10" {
					t.Errorf("sortCode: expected %q, got %q", "10-10-10", body.SortCode)
				}
				if body.Balance != 0.00 {
					t.Errorf("balance: expected 0.00, got %v", body.Balance)
				}
				if body.Currency != "GBP" {
					t.Errorf("currency: expected %q, got %q", "GBP", body.Currency)
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
