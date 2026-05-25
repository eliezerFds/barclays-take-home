package server

import (
	"context"
	"errors"

	"barclays/internal/storage"
	"github.com/danielgtaylor/huma/v2"
)

type CreateAccountRequest struct {
	Body CreateAccountRequestBody
}

type CreateAccountRequestBody struct {
	Name        string `json:"name" required:"true" doc:"Account name"`
	AccountType string `json:"accountType" required:"true" enum:"personal"`
}

type AccountResponseBody struct {
	AccountNumber    string  `json:"accountNumber"`
	SortCode         string  `json:"sortCode"`
	Name             string  `json:"name"`
	AccountType      string  `json:"accountType"`
	Balance          float64 `json:"balance"`
	Currency         string  `json:"currency"`
	CreatedTimestamp string  `json:"createdTimestamp"`
	UpdatedTimestamp string  `json:"updatedTimestamp"`
}

type AccountResponse struct {
	Body AccountResponseBody
}

func toAccountResponse(a *storage.Account) *AccountResponse {
	return &AccountResponse{
		Body: AccountResponseBody{
			AccountNumber:    a.AccountNumber,
			SortCode:         a.SortCode,
			Name:             a.Name,
			AccountType:      a.AccountType,
			Balance:          float64(a.Balance) / 100,
			Currency:         a.Currency,
			CreatedTimestamp: a.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedTimestamp: a.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		},
	}
}

type FetchAccountRequest struct {
	AccountNumber string `path:"accountNumber" pattern:"^01\\d{6}$" doc:"Account number"`
}

func (s *Server) FetchAccount(ctx context.Context, req *FetchAccountRequest) (*AccountResponse, error) {
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

	return toAccountResponse(account), nil
}

func (s *Server) CreateAccount(ctx context.Context, req *CreateAccountRequest) (*AccountResponse, error) {
	userID, err := getAuthenticatedUserID(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}

	account, err := s.repository.CreateAccount(ctx, storage.CreateAccountParams{
		Name:        req.Body.Name,
		AccountType: req.Body.AccountType,
		UserID:      userID,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create account")
	}

	return toAccountResponse(account), nil
}
