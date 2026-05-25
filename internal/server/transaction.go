package server

import (
	"context"
	"errors"
	"math"

	"barclays/internal/storage"

	"github.com/danielgtaylor/huma/v2"
)

// --- Request / Response types ---

type CreateTransactionRequest struct {
	AccountNumber string `path:"accountNumber" pattern:"^01\\d{6}$" doc:"Account number"`
	Body          CreateTransactionRequestBody
}

type CreateTransactionRequestBody struct {
	Amount    float64 `json:"amount" required:"true" minimum:"0" maximum:"10000" doc:"Transaction amount"`
	Currency  string  `json:"currency" required:"true" enum:"GBP"`
	Type      string  `json:"type" required:"true" enum:"deposit,withdrawal"`
	Reference string  `json:"reference,omitempty"`
}

type FetchTransactionRequest struct {
	AccountNumber string `path:"accountNumber" pattern:"^01\\d{6}$" doc:"Account number"`
	// The spec pattern ^tan-[A-Za-z0-9]$ is missing the + quantifier; using ^tan-[A-Za-z0-9]+$ to match actual IDs.
	TransactionID string `path:"transactionId" pattern:"^tan-[A-Za-z0-9]+$" doc:"Transaction ID"`
}

type TransactionResponseBody struct {
	ID               string  `json:"id"`
	Amount           float64 `json:"amount"`
	Currency         string  `json:"currency"`
	Type             string  `json:"type"`
	Reference        string  `json:"reference,omitempty"`
	UserID           string  `json:"userId"`
	CreatedTimestamp string  `json:"createdTimestamp"`
}

type TransactionResponse struct {
	Body TransactionResponseBody
}

func toTransactionBody(t *storage.Transaction, userID string) TransactionResponseBody {
	body := TransactionResponseBody{
		ID:               t.ID,
		Amount:           float64(t.Amount) / 100,
		Currency:         t.Currency,
		Type:             t.Type,
		Reference:        t.Reference,
		UserID:           userID,
		CreatedTimestamp: t.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	return body
}

// --- Handlers ---

func (s *Server) CreateTransaction(ctx context.Context, req *CreateTransactionRequest) (*TransactionResponse, error) {
	account, err := s.repository.GetAccountByNumber(ctx, req.AccountNumber)
	if err != nil {
		if errors.Is(err, storage.ErrAccountNotFound) {
			return nil, huma.Error404NotFound("account not found")
		}
		return nil, huma.Error500InternalServerError("failed to fetch account")
	}

	callerID, err := getAuthenticatedUserID(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	if callerID != account.UserID {
		return nil, huma.Error403Forbidden("you are not allowed to access this account")
	}

	transaction, err := s.repository.CreateTransaction(ctx, storage.CreateTransactionParams{
		AccountNumber: req.AccountNumber,
		Amount:        int64(math.Round(req.Body.Amount * 100)),
		Currency:      req.Body.Currency,
		Type:          req.Body.Type,
		Reference:     req.Body.Reference,
	})
	if err != nil {
		if errors.Is(err, storage.ErrInsufficientFunds) {
			return nil, &huma.ErrorModel{Status: 422, Title: "Unprocessable Entity", Detail: "insufficient funds"}
		}
		return nil, huma.Error500InternalServerError("failed to create transaction")
	}

	return &TransactionResponse{Body: toTransactionBody(transaction, account.UserID)}, nil
}

func (s *Server) FetchTransaction(ctx context.Context, req *FetchTransactionRequest) (*TransactionResponse, error) {
	account, err := s.repository.GetAccountByNumber(ctx, req.AccountNumber)
	if err != nil {
		if errors.Is(err, storage.ErrAccountNotFound) {
			return nil, huma.Error404NotFound("account not found")
		}
		return nil, huma.Error500InternalServerError("failed to fetch account")
	}

	callerID, err := getAuthenticatedUserID(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	if callerID != account.UserID {
		return nil, huma.Error403Forbidden("you are not allowed to access this account")
	}

	transaction, err := s.repository.GetTransactionByID(ctx, req.AccountNumber, req.TransactionID)
	if err != nil {
		if errors.Is(err, storage.ErrTransactionNotFound) {
			return nil, huma.Error404NotFound("transaction not found")
		}
		return nil, huma.Error500InternalServerError("failed to fetch transaction")
	}

	return &TransactionResponse{Body: toTransactionBody(transaction, account.UserID)}, nil
}
