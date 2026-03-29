package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/cory-johannsen/mud/cmd/webclient/handlers"
	"github.com/cory-johannsen/mud/cmd/webclient/middleware"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

const testJWTSecret = "test-jwt-secret"

// fakeAccountStore is a test double for handlers.AccountStore.
type fakeAccountStore struct {
	createFn       func(ctx context.Context, username, password string) (postgres.Account, error)
	authenticateFn func(ctx context.Context, username, password string) (postgres.Account, error)
	getByIDFn      func(ctx context.Context, id int64) (postgres.Account, error)
}

func (f *fakeAccountStore) Create(ctx context.Context, username, password string) (postgres.Account, error) {
	if f.createFn != nil {
		return f.createFn(ctx, username, password)
	}
	return postgres.Account{}, errors.New("Create not configured")
}
func (f *fakeAccountStore) Authenticate(ctx context.Context, username, password string) (postgres.Account, error) {
	if f.authenticateFn != nil {
		return f.authenticateFn(ctx, username, password)
	}
	return postgres.Account{}, errors.New("Authenticate not configured")
}
func (f *fakeAccountStore) GetByID(ctx context.Context, id int64) (postgres.Account, error) {
	if f.getByIDFn != nil {
		return f.getByIDFn(ctx, id)
	}
	return postgres.Account{}, errors.New("GetByID not configured")
}

func newTestHandler(store handlers.AccountStore) *handlers.AuthHandler {
	return handlers.NewAuthHandler(store, testJWTSecret)
}

func decodeTokenResponse(t *testing.T, body *bytes.Buffer) handlers.TokenResponse {
	t.Helper()
	var resp handlers.TokenResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	return resp
}

func validateJWT(t *testing.T, tokenStr string, wantAccountID int64, wantRole string) {
	t.Helper()
	tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	})
	if err != nil || !tok.Valid {
		t.Fatalf("invalid JWT: %v", err)
	}
	claims := tok.Claims.(jwt.MapClaims)
	if int64(claims["account_id"].(float64)) != wantAccountID {
		t.Errorf("account_id = %v, want %d", claims["account_id"], wantAccountID)
	}
	if claims["role"].(string) != wantRole {
		t.Errorf("role = %v, want %s", claims["role"], wantRole)
	}
	exp := int64(claims["exp"].(float64))
	if exp < time.Now().Unix() {
		t.Error("token already expired")
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name        string
		body        any
		storeFn     func(ctx context.Context, u, p string) (postgres.Account, error)
		wantStatus  int
		wantTokenID int64
	}{
		{
			name: "success",
			body: map[string]string{"username": "alice", "password": "password123"},
			storeFn: func(_ context.Context, _, _ string) (postgres.Account, error) {
				return postgres.Account{ID: 1, Username: "alice", Role: "player"}, nil
			},
			wantStatus:  http.StatusOK,
			wantTokenID: 1,
		},
		{
			name: "invalid credentials",
			body: map[string]string{"username": "alice", "password": "wrong"},
			storeFn: func(_ context.Context, _, _ string) (postgres.Account, error) {
				return postgres.Account{}, postgres.ErrInvalidCredentials
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "malformed body",
			body:       "not json",
			storeFn:    nil,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeAccountStore{
				authenticateFn: tt.storeFn,
			}
			h := newTestHandler(store)

			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.Login(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				resp := decodeTokenResponse(t, rr.Body)
				validateJWT(t, resp.Token, tt.wantTokenID, "player")
			}
		})
	}
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name       string
		body       map[string]string
		storeFn    func(ctx context.Context, u, p string) (postgres.Account, error)
		wantStatus int
	}{
		{
			name: "success",
			body: map[string]string{"username": "bob_23", "password": "securepassword"},
			storeFn: func(_ context.Context, _, _ string) (postgres.Account, error) {
				return postgres.Account{ID: 2, Username: "bob_23", Role: "player"}, nil
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "username too short",
			body:       map[string]string{"username": "ab", "password": "securepassword"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "username too long",
			body:       map[string]string{"username": "abcdefghijklmnopqrstu", "password": "securepassword"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "username invalid chars",
			body:       map[string]string{"username": "ab-cd", "password": "securepassword"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "password too short",
			body:       map[string]string{"username": "alice", "password": "short"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "username already taken",
			body: map[string]string{"username": "alice", "password": "securepassword"},
			storeFn: func(_ context.Context, _, _ string) (postgres.Account, error) {
				return postgres.Account{}, postgres.ErrAccountExists
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeAccountStore{
				createFn: tt.storeFn,
			}
			h := newTestHandler(store)

			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.Register(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestMe(t *testing.T) {
	store := &fakeAccountStore{
		getByIDFn: func(_ context.Context, id int64) (postgres.Account, error) {
			if id == 5 {
				return postgres.Account{ID: 5, Username: "charlie", Role: "admin"}, nil
			}
			return postgres.Account{}, postgres.ErrAccountNotFound
		},
	}
	h := newTestHandler(store)

	t.Run("returns account for authenticated user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		ctx := context.WithValue(req.Context(), middleware.ClaimsKey, middleware.Claims{AccountID: 5, Role: "admin"})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		h.Me(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rr.Code)
		}
		var resp map[string]any
		_ = json.NewDecoder(rr.Body).Decode(&resp)
		if resp["username"] != "charlie" {
			t.Errorf("username = %v, want charlie", resp["username"])
		}
	})

	t.Run("401 when no claims in context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		rr := httptest.NewRecorder()
		h.Me(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rr.Code)
		}
	})
}
