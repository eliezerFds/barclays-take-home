package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// createTransaction posts a transaction against accountNumber and returns the response body.
func createTransaction(t *testing.T, ts *httptest.Server, token, accountNumber, body string) (int, TransactionResponseBody) {
	t.Helper()
	client := ts.Client()
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/v1/accounts/%s/transactions", ts.URL, accountNumber),
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result TransactionResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("createTransaction: failed to decode response body: %v", err)
	}
	return resp.StatusCode, result
}

func TestCreateTransaction(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	client := ts.Client()

	_, token := createAndLogin(t, ts, "tx-create@example.com")
	_, otherToken := createAndLogin(t, ts, "tx-create-other@example.com")
	accountNumber := createAccount(t, ts, token)

	// Seed a deposit so withdrawal tests have funds to draw from.
	if code, _ := createTransaction(t, ts, token, accountNumber, `{"amount":100,"currency":"GBP","type":"deposit"}`); code != http.StatusCreated {
		t.Fatalf("seed deposit failed: %d", code)
	}

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

	txURL := fmt.Sprintf("%s/v1/accounts/%s/transactions", ts.URL, accountNumber)

	tests := []struct {
		name           string
		url            string
		body           string
		token          string
		expectedStatus int
	}{
		{
			name:           "success - deposit",
			url:            txURL,
			body:           `{"amount":50.00,"currency":"GBP","type":"deposit"}`,
			token:          token,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "success - withdrawal",
			url:            txURL,
			body:           `{"amount":10.00,"currency":"GBP","type":"withdrawal"}`,
			token:          token,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "success - deposit with reference",
			url:            txURL,
			body:           `{"amount":25.00,"currency":"GBP","type":"deposit","reference":"salary"}`,
			token:          token,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "insufficient funds",
			url:            txURL,
			body:           `{"amount":9999.99,"currency":"GBP","type":"withdrawal"}`,
			token:          token,
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "missing amount",
			url:            txURL,
			body:           `{"currency":"GBP","type":"deposit"}`,
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing currency",
			url:            txURL,
			body:           `{"amount":10.00,"type":"deposit"}`,
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid currency",
			url:            txURL,
			body:           `{"amount":10.00,"currency":"USD","type":"deposit"}`,
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing type",
			url:            txURL,
			body:           `{"amount":10.00,"currency":"GBP"}`,
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid type",
			url:            txURL,
			body:           `{"amount":10.00,"currency":"GBP","type":"transfer"}`,
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "zero amount",
			url:            txURL,
			body:           `{"amount":0,"currency":"GBP","type":"deposit"}`,
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "account not found",
			url:            fmt.Sprintf("%s/v1/accounts/01000000/transactions", ts.URL),
			body:           `{"amount":10.00,"currency":"GBP","type":"deposit"}`,
			token:          token,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "forbidden - another user's account",
			url:            txURL,
			body:           `{"amount":10.00,"currency":"GBP","type":"deposit"}`,
			token:          otherToken,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "unauthorised - no token",
			url:            txURL,
			body:           `{"amount":10.00,"currency":"GBP","type":"deposit"}`,
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := authPost(tc.url, tc.body, tc.token)
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			if tc.expectedStatus == http.StatusCreated {
				var body TransactionResponseBody
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.ID == "" {
					t.Error("expected non-empty id")
				}
				if body.Currency != "GBP" {
					t.Errorf("currency: expected %q, got %q", "GBP", body.Currency)
				}
				if body.UserID == "" {
					t.Error("expected non-empty userId")
				}
				if body.CreatedTimestamp == "" {
					t.Error("expected non-empty createdTimestamp")
				}
			}
		})
	}
}

func TestTransactionUpdatesBalance(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	client := ts.Client()

	_, token := createAndLogin(t, ts, "balance-check@example.com")
	accountNumber := createAccount(t, ts, token)

	getBalance := func() float64 {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/accounts/"+accountNumber, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var body AccountResponseBody
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		return body.Balance
	}

	if bal := getBalance(); bal != 0 {
		t.Fatalf("expected initial balance 0, got %v", bal)
	}

	if code, _ := createTransaction(t, ts, token, accountNumber, `{"amount":100.00,"currency":"GBP","type":"deposit"}`); code != http.StatusCreated {
		t.Fatalf("deposit failed: %d", code)
	}
	if bal := getBalance(); bal != 100.00 {
		t.Errorf("after £100 deposit: expected balance 100.00, got %v", bal)
	}

	if code, _ := createTransaction(t, ts, token, accountNumber, `{"amount":40.50,"currency":"GBP","type":"withdrawal"}`); code != http.StatusCreated {
		t.Fatalf("withdrawal failed: %d", code)
	}
	if bal := getBalance(); bal != 59.50 {
		t.Errorf("after £40.50 withdrawal: expected balance 59.50, got %v", bal)
	}

	if code, _ := createTransaction(t, ts, token, accountNumber, `{"amount":59.50,"currency":"GBP","type":"withdrawal"}`); code != http.StatusCreated {
		t.Fatalf("exact withdrawal failed: %d", code)
	}
	if bal := getBalance(); bal != 0 {
		t.Errorf("after withdrawing remaining balance: expected balance 0, got %v", bal)
	}
}

func TestFetchTransaction(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	client := ts.Client()

	_, token := createAndLogin(t, ts, "tx-fetch@example.com")
	_, otherToken := createAndLogin(t, ts, "tx-fetch-other@example.com")
	accountNumber := createAccount(t, ts, token)

	_, tx := createTransaction(t, ts, token, accountNumber, `{"amount":50,"currency":"GBP","type":"deposit","reference":"test ref"}`)
	transactionID := tx.ID

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

	txURL := fmt.Sprintf("%s/v1/accounts/%s/transactions/%s", ts.URL, accountNumber, transactionID)

	tests := []struct {
		name           string
		url            string
		token          string
		expectedStatus int
	}{
		{
			name:           "success - fetch own transaction",
			url:            txURL,
			token:          token,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "forbidden - another user's account",
			url:            txURL,
			token:          otherToken,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "not found - non-existent transaction",
			url:            fmt.Sprintf("%s/v1/accounts/%s/transactions/tan-doesnotexist", ts.URL, accountNumber),
			token:          token,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "not found - non-existent account",
			url:            fmt.Sprintf("%s/v1/accounts/01000000/transactions/%s", ts.URL, transactionID),
			token:          token,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "unauthorised - no token",
			url:            txURL,
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := authGet(tc.url, tc.token)
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			if tc.expectedStatus == http.StatusOK {
				var body TransactionResponseBody
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.ID != transactionID {
					t.Errorf("id: expected %q, got %q", transactionID, body.ID)
				}
				if body.Amount != 50 {
					t.Errorf("amount: expected 50, got %v", body.Amount)
				}
				if body.Type != "deposit" {
					t.Errorf("type: expected %q, got %q", "deposit", body.Type)
				}
				if body.Reference != "test ref" {
					t.Errorf("reference: expected %q, got %q", "test ref", body.Reference)
				}
				if body.Currency != "GBP" {
					t.Errorf("currency: expected %q, got %q", "GBP", body.Currency)
				}
				if body.UserID == "" {
					t.Error("expected non-empty userId")
				}
				if body.CreatedTimestamp == "" {
					t.Error("expected non-empty createdTimestamp")
				}
			}
		})
	}
}
