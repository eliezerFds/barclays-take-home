package server

import (
	"context"
	"errors"

	"barclays/internal/storage"
	"github.com/danielgtaylor/huma/v2"
)

type CreateUserRequest struct {
	Body CreateUserRequestBody
}

type CreateUserRequestBody struct {
	Name        string      `json:"name" required:"true" doc:"Full name of the user"`
	Address     UserAddress `json:"address" required:"true"`
	PhoneNumber string      `json:"phoneNumber" required:"true" doc:"E.164 format phone number"`
	Email       string      `json:"email" required:"true" format:"email"`
	Password    string      `json:"password" required:"true" minLength:"8"`
}

type UserAddress struct {
	Line1    string `json:"line1" required:"true"`
	Line2    string `json:"line2,omitempty"`
	Line3    string `json:"line3,omitempty"`
	Town     string `json:"town" required:"true"`
	County   string `json:"county" required:"true"`
	Postcode string `json:"postcode" required:"true"`
}

type UserResponse struct {
	Body UserResponseBody
}

type UserResponseBody struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	Address          UserAddress `json:"address"`
	PhoneNumber      string      `json:"phoneNumber"`
	Email            string      `json:"email"`
	CreatedTimestamp string      `json:"createdTimestamp"`
	UpdatedTimestamp string      `json:"updatedTimestamp"`
}

func toUserResponse(u *storage.User) *UserResponse {
	return &UserResponse{
		Body: UserResponseBody{
			ID:   u.ID,
			Name: u.Name,
			Address: UserAddress{
				Line1:    u.AddressLine1,
				Line2:    u.AddressLine2,
				Line3:    u.AddressLine3,
				Town:     u.AddressTown,
				County:   u.AddressCounty,
				Postcode: u.AddressPostcode,
			},
			PhoneNumber:      u.PhoneNumber,
			Email:            u.Email,
			CreatedTimestamp: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedTimestamp: u.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		},
	}
}

type FetchUserRequest struct {
	UserID string `path:"userId" pattern:"^usr-[A-Za-z0-9]+$" doc:"User ID"`
}

func (s *Server) CreateUser(ctx context.Context, req *CreateUserRequest) (*UserResponse, error) {
	passwordHash, err := hashPassword(req.Body.Password)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to process request")
	}

	user, err := s.repository.CreateUser(ctx, storage.CreateUserParams{
		Name:            req.Body.Name,
		AddressLine1:    req.Body.Address.Line1,
		AddressLine2:    req.Body.Address.Line2,
		AddressLine3:    req.Body.Address.Line3,
		AddressTown:     req.Body.Address.Town,
		AddressCounty:   req.Body.Address.County,
		AddressPostcode: req.Body.Address.Postcode,
		PhoneNumber:     req.Body.PhoneNumber,
		Email:           req.Body.Email,
		PasswordHash:    passwordHash,
	})
	if err != nil {
		if errors.Is(err, storage.ErrDuplicateUser) {
			return nil, huma.Error409Conflict("a user with this email already exists")
		}
		return nil, huma.Error500InternalServerError("failed to create user")
	}

	return toUserResponse(user), nil
}

func (s *Server) FetchUser(ctx context.Context, req *FetchUserRequest) (*UserResponse, error) {
	user, err := s.repository.GetUserByID(ctx, req.UserID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, huma.Error404NotFound("user not found")
		}
		return nil, huma.Error500InternalServerError("failed to fetch user")
	}

	if getAuthenticatedUserID(ctx) != user.ID {
		return nil, huma.Error403Forbidden("you are not allowed to access this user")
	}

	return toUserResponse(user), nil
}
