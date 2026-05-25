package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// TestFullJourney walks through the complete happy path end-to-end:
// register → login → fetch profile → create account → deposit → check balance
// → withdraw → fetch transaction → check balance → attempt overdraft.
func TestFullJourney(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	client := ts.Client()

	req := func(method, url, token, body string) *http.Response {
		t.Helper()
		var r *http.Request
		if body != "" {
			r, _ = http.NewRequest(method, url, bytes.NewBufferString(body))
			r.Header.Set("Content-Type", "application/json")
		} else {
			r, _ = http.NewRequest(method, url, nil)
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	mustStatus := func(resp *http.Response, want int) {
		t.Helper()
		if resp.StatusCode != want {
			t.Fatalf("%s %s: expected status %d, got %d", resp.Request.Method, resp.Request.URL, want, resp.StatusCode)
		}
	}

	// 1. Register.
	resp := req(http.MethodPost, ts.URL+"/v1/users", "", `{
		"name":"Jane Smith",
		"address":{"line1":"10 Downing Street","town":"London","county":"Greater London","postcode":"SW1A 2AA"},
		"phoneNumber":"+447700900000",
		"email":"jane@example.com",
		"password":"securepass1"
	}`)
	mustStatus(resp, http.StatusCreated)
	var user UserResponseBody
	json.NewDecoder(resp.Body).Decode(&user)
	resp.Body.Close()

	// 2. Login.
	resp = req(http.MethodPost, ts.URL+"/v1/auth/login", "", `{"email":"jane@example.com","password":"securepass1"}`)
	mustStatus(resp, http.StatusOK)
	var login LoginResponseBody
	json.NewDecoder(resp.Body).Decode(&login)
	resp.Body.Close()
	token := login.Token

	// 3. Fetch own profile.
	resp = req(http.MethodGet, fmt.Sprintf("%s/v1/users/%s", ts.URL, user.ID), token, "")
	mustStatus(resp, http.StatusOK)
	var profile UserResponseBody
	json.NewDecoder(resp.Body).Decode(&profile)
	resp.Body.Close()
	if profile.Email != "jane@example.com" {
		t.Errorf("profile email: got %q, want jane@example.com", profile.Email)
	}

	// 4. Create a bank account.
	resp = req(http.MethodPost, ts.URL+"/v1/accounts", token, `{"name":"Current Account","accountType":"personal"}`)
	mustStatus(resp, http.StatusCreated)
	var account AccountResponseBody
	json.NewDecoder(resp.Body).Decode(&account)
	resp.Body.Close()
	if account.Balance != 0 {
		t.Errorf("new account balance: got %v, want 0", account.Balance)
	}

	txURL := fmt.Sprintf("%s/v1/accounts/%s/transactions", ts.URL, account.AccountNumber)
	accountURL := fmt.Sprintf("%s/v1/accounts/%s", ts.URL, account.AccountNumber)

	// 5. Deposit £100.
	resp = req(http.MethodPost, txURL, token, `{"amount":100.00,"currency":"GBP","type":"deposit","reference":"initial deposit"}`)
	mustStatus(resp, http.StatusCreated)
	var depositTx TransactionResponseBody
	json.NewDecoder(resp.Body).Decode(&depositTx)
	resp.Body.Close()
	if depositTx.Amount != 100.00 {
		t.Errorf("deposit amount: got %v, want 100.00", depositTx.Amount)
	}
	if depositTx.Type != "deposit" {
		t.Errorf("deposit type: got %q, want deposit", depositTx.Type)
	}
	if depositTx.UserID != user.ID {
		t.Errorf("deposit userId: got %q, want %q", depositTx.UserID, user.ID)
	}

	// 6. Verify balance is £100.
	resp = req(http.MethodGet, accountURL, token, "")
	mustStatus(resp, http.StatusOK)
	json.NewDecoder(resp.Body).Decode(&account)
	resp.Body.Close()
	if account.Balance != 100.00 {
		t.Errorf("balance after deposit: got %v, want 100.00", account.Balance)
	}

	// 7. Withdraw £40.50.
	resp = req(http.MethodPost, txURL, token, `{"amount":40.50,"currency":"GBP","type":"withdrawal","reference":"groceries"}`)
	mustStatus(resp, http.StatusCreated)
	var withdrawTx TransactionResponseBody
	json.NewDecoder(resp.Body).Decode(&withdrawTx)
	resp.Body.Close()
	if withdrawTx.Amount != 40.50 {
		t.Errorf("withdrawal amount: got %v, want 40.50", withdrawTx.Amount)
	}

	// 8. Fetch the withdrawal transaction by ID.
	resp = req(http.MethodGet, fmt.Sprintf("%s/v1/accounts/%s/transactions/%s", ts.URL, account.AccountNumber, withdrawTx.ID), token, "")
	mustStatus(resp, http.StatusOK)
	var fetched TransactionResponseBody
	json.NewDecoder(resp.Body).Decode(&fetched)
	resp.Body.Close()
	if fetched.ID != withdrawTx.ID {
		t.Errorf("fetched tx id: got %q, want %q", fetched.ID, withdrawTx.ID)
	}
	if fetched.Reference != "groceries" {
		t.Errorf("fetched tx reference: got %q, want groceries", fetched.Reference)
	}

	// 9. Verify balance is £59.50.
	resp = req(http.MethodGet, accountURL, token, "")
	mustStatus(resp, http.StatusOK)
	json.NewDecoder(resp.Body).Decode(&account)
	resp.Body.Close()
	if account.Balance != 59.50 {
		t.Errorf("balance after withdrawal: got %v, want 59.50", account.Balance)
	}

	// 10. Overdraft must be rejected with 422.
	resp = req(http.MethodPost, txURL, token, `{"amount":9999.00,"currency":"GBP","type":"withdrawal"}`)
	mustStatus(resp, http.StatusUnprocessableEntity)
	resp.Body.Close()

	// 11. Balance must be unchanged after failed overdraft.
	resp = req(http.MethodGet, accountURL, token, "")
	mustStatus(resp, http.StatusOK)
	json.NewDecoder(resp.Body).Decode(&account)
	resp.Body.Close()
	if account.Balance != 59.50 {
		t.Errorf("balance after failed overdraft: got %v, want 59.50", account.Balance)
	}
}

// TestCrossUserAuthorisation ensures users cannot access each other's resources.
func TestCrossUserAuthorisation(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	_, tokenA := createAndLogin(t, ts, "user-a@example.com")
	_, tokenB := createAndLogin(t, ts, "user-b@example.com")
	accountA := createAccount(t, ts, tokenA)

	authReq := func(method, url, token string) int {
		t.Helper()
		r, _ := http.NewRequest(method, url, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		resp, err := ts.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	// User B cannot fetch user A's account.
	if code := authReq(http.MethodGet, fmt.Sprintf("%s/v1/accounts/%s", ts.URL, accountA), tokenB); code != http.StatusForbidden {
		t.Errorf("cross-user fetch account: got %d, want 403", code)
	}

	// Seed funds for account A.
	createTransaction(t, ts, tokenA, accountA, `{"amount":50.00,"currency":"GBP","type":"deposit"}`)

	// User B cannot transact on user A's account.
	r, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/v1/accounts/%s/transactions", ts.URL, accountA),
		bytes.NewBufferString(`{"amount":10.00,"currency":"GBP","type":"withdrawal"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+tokenB)
	resp, _ := ts.Client().Do(r)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-user create transaction: got %d, want 403", resp.StatusCode)
	}
}

// TestUnauthenticatedAccess verifies all protected endpoints reject requests without a token.
func TestUnauthenticatedAccess(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	_, token := createAndLogin(t, ts, "auth-test@example.com")
	accountNumber := createAccount(t, ts, token)
	_, tx := createTransaction(t, ts, token, accountNumber, `{"amount":10.00,"currency":"GBP","type":"deposit"}`)

	endpoints := []struct {
		method string
		url    string
	}{
		{http.MethodGet, fmt.Sprintf("%s/v1/users/usr-abc123", ts.URL)},
		{http.MethodPost, fmt.Sprintf("%s/v1/accounts", ts.URL)},
		{http.MethodGet, fmt.Sprintf("%s/v1/accounts/%s", ts.URL, accountNumber)},
		{http.MethodPost, fmt.Sprintf("%s/v1/accounts/%s/transactions", ts.URL, accountNumber)},
		{http.MethodGet, fmt.Sprintf("%s/v1/accounts/%s/transactions/%s", ts.URL, accountNumber, tx.ID)},
	}

	for _, e := range endpoints {
		r, _ := http.NewRequest(e.method, e.url, nil)
		resp, err := ts.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401", e.method, e.url, resp.StatusCode)
		}
	}
}
