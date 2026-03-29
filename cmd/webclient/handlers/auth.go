// Package handlers provides HTTP request handlers for the web client API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/cory-johannsen/mud/cmd/webclient/middleware"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// AccountStore is the persistence interface required by AuthHandler.
type AccountStore interface {
	Create(ctx context.Context, username, password string) (postgres.Account, error)
	Authenticate(ctx context.Context, username, password string) (postgres.Account, error)
	GetByID(ctx context.Context, id int64) (postgres.Account, error)
}

// TokenResponse is the JSON body returned on successful authentication.
type TokenResponse struct {
	Token     string `json:"token"`
	AccountID int64  `json:"account_id"`
	Role      string `json:"role"`
}

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_]{3,20}$`)

// AuthHandler handles authentication API endpoints.
type AuthHandler struct {
	store     AccountStore
	jwtSecret string
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(store AccountStore, jwtSecret string) *AuthHandler {
	return &AuthHandler{store: store, jwtSecret: jwtSecret}
}

// Login handles POST /api/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	acct, err := h.store.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, postgres.ErrInvalidCredentials) || errors.Is(err, postgres.ErrAccountNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "authentication error")
		return
	}
	writeTokenResponse(w, http.StatusOK, acct, h.jwtSecret)
}

// Register handles POST /api/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !usernameRe.MatchString(req.Username) {
		writeError(w, http.StatusBadRequest, "username must be 3-20 alphanumeric characters or underscores")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	acct, err := h.store.Create(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, postgres.ErrAccountExists) {
			writeError(w, http.StatusConflict, "username already taken")
			return
		}
		writeError(w, http.StatusInternalServerError, "registration error")
		return
	}
	writeTokenResponse(w, http.StatusCreated, acct, h.jwtSecret)
}

// Me handles GET /api/auth/me.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	acct, err := h.store.GetByID(r.Context(), claims.AccountID)
	if err != nil {
		if errors.Is(err, postgres.ErrAccountNotFound) {
			writeError(w, http.StatusUnauthorized, "account not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"account_id": acct.ID,
		"username":   acct.Username,
		"role":       acct.Role,
	})
}

func writeTokenResponse(w http.ResponseWriter, status int, acct postgres.Account, secret string) {
	token, err := issueJWT(acct, secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(TokenResponse{
		Token:     token,
		AccountID: acct.ID,
		Role:      acct.Role,
	})
}

func issueJWT(acct postgres.Account, secret string) (string, error) {
	claims := jwt.MapClaims{
		"account_id": acct.ID,
		"role":       acct.Role,
		"exp":        time.Now().Add(24 * time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(secret))
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
