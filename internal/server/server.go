package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"barclays/internal/storage"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

type Dependencies struct {
	Repository *storage.Storage
}

type Server struct {
	routes     http.Handler
	repository *storage.Storage
}

func (s *Server) Start(port int) {
	log.Printf("server started. listening on port %d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), s.routes); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// publicRoutes lists routes that do not require a JWT token.
// Key: path, value: HTTP method.
var publicRoutes = map[string]string{
	"/v1/users":       http.MethodPost,
	"/v1/auth/login":  http.MethodPost,
}

func New(deps Dependencies) *Server {
	router := http.NewServeMux()
	api := humago.New(router, huma.DefaultConfig("Eagle Bank API", "v1.0.0"))

	// Huma returns 422 for validation errors by default; the spec requires 400.
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		if status == http.StatusUnprocessableEntity {
			status = http.StatusBadRequest
		}
		out := &huma.ErrorModel{Status: status, Title: http.StatusText(status), Detail: msg}
		for _, err := range errs {
			var detail *huma.ErrorDetail
			if errors.As(err, &detail) {
				out.Errors = append(out.Errors, detail)
			} else {
				out.Errors = append(out.Errors, &huma.ErrorDetail{Message: err.Error()})
			}
		}
		return out
	}

	s := &Server{
		repository: deps.Repository,
	}

	// Public routes
	huma.Post(api, "/v1/users", s.CreateUser, func(o *huma.Operation) {
		o.DefaultStatus = http.StatusCreated
	})
	huma.Post(api, "/v1/auth/login", s.Login)

	// Protected routes
	huma.Get(api, "/v1/users/{userId}", s.FetchUser)

	s.routes = selectiveAuthMiddleware(router)

	return s
}

// selectiveAuthMiddleware applies JWT auth to all routes except those listed in publicRoutes.
func selectiveAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if method, ok := publicRoutes[r.URL.Path]; ok && r.Method == method {
			next.ServeHTTP(w, r)
			return
		}
		jwtMiddleware(next).ServeHTTP(w, r)
	})
}
