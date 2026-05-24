package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"barclays/internal/storage"

	"github.com/danielgtaylor/huma/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const authenticatedUserIDKey contextKey = "authenticatedUserID"

var jwtSecret = func() []byte {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte("dev-secret")
}()

// --- Login ---

type LoginRequest struct {
	Body LoginRequestBody
}

type LoginRequestBody struct {
	Email    string `json:"email" required:"true" format:"email"`
	Password string `json:"password" required:"true"`
}

type LoginResponse struct {
	Body LoginResponseBody
}

type LoginResponseBody struct {
	Token string `json:"token" doc:"JWT bearer token"`
}

func (s *Server) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	user, err := s.repository.GetUserByEmail(ctx, req.Body.Email)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, huma.Error401Unauthorized("invalid credentials")
		}
		return nil, huma.Error500InternalServerError("failed to process request")
	}

	if !checkPassword(user.PasswordHash, req.Body.Password) {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}

	token, err := generateToken(user.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to generate token")
	}

	return &LoginResponse{Body: LoginResponseBody{Token: token}}, nil
}

// --- JWT helpers ---

func generateToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
}

func parseToken(tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return "", errors.New("invalid token")
	}
	sub, err := token.Claims.GetSubject()
	if err != nil || sub == "" {
		return "", errors.New("invalid token claims")
	}
	return sub, nil
}

// --- Middleware ---

func jwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeUnauthorized(w)
			return
		}

		userID, err := parseToken(strings.TrimPrefix(authHeader, "Bearer "))
		if err != nil {
			writeUnauthorized(w)
			return
		}

		ctx := context.WithValue(r.Context(), authenticatedUserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"message":"access token is missing or invalid"}`)) //nolint:errcheck
}

// --- Password helpers ---

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func getAuthenticatedUserID(ctx context.Context) string {
	id := ctx.Value(authenticatedUserIDKey).(string)
	return id
}
