# Web Client Phase 1: Go HTTP Server & Auth API

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the cmd/webclient Go binary with HTTP server, JWT auth API, and config wiring.

**Architecture:** Standard net/http server with embedded React SPA, JWT middleware, PostgreSQL-backed auth. No external HTTP frameworks.

**Tech Stack:** Go net/http, golang-jwt/jwt/v5, pgx v5, Wire DI

---

## Requirements Covered

REQ-WC-1, REQ-WC-2, REQ-WC-3, REQ-WC-4, REQ-WC-5, REQ-WC-6, REQ-WC-7, REQ-WC-8, REQ-WC-9, REQ-WC-10, REQ-WC-11

## File Structure

```
internal/config/config.go              (modify: add WebConfig + validateWeb)
cmd/webclient/
  main.go                              (create: entry point, signal handling, graceful shutdown)
  server.go                            (create: Server struct, routing, static file serving)
  config_test.go                       (create: WebConfig validation tests)
  middleware/
    jwt.go                             (create: JWT validation middleware)
    jwt_test.go                        (create: middleware tests)
  handlers/
    auth.go                            (create: login, register, me handlers + AccountStore interface)
    auth_test.go                       (create: table-driven handler tests)
```

---

## Prerequisites

- [ ] Verify `github.com/golang-jwt/jwt/v5` is in `go.mod`. If absent, run:
  ```
  mise exec -- go get github.com/golang-jwt/jwt/v5
  ```
- [ ] Verify `nhooyr.io/websocket` or `github.com/gorilla/websocket` is available (Phase 4 dependency; add `nhooyr.io/websocket` now if absent):
  ```
  mise exec -- go get nhooyr.io/websocket
  ```

---

## Task 1: Add WebConfig to internal/config/config.go

**REQ-WC-6**

### Step 1.1 — Write failing test

Create `internal/config/web_config_test.go`:

```go
package config_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/config"
)

func TestWebConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.WebConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     config.WebConfig{Port: 8080, JWTSecret: "supersecret"},
			wantErr: false,
		},
		{
			name:    "port zero uses default",
			cfg:     config.WebConfig{Port: 0, JWTSecret: "supersecret"},
			wantErr: false,
		},
		{
			name:    "port out of range high",
			cfg:     config.WebConfig{Port: 99999, JWTSecret: "supersecret"},
			wantErr: true,
		},
		{
			name:    "empty jwt secret",
			cfg:     config.WebConfig{Port: 8080, JWTSecret: ""},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

Run (expect failure):
```
mise exec -- go test ./internal/config/... -v -run TestWebConfig_Validate
```

### Step 1.2 — Implement

In `internal/config/config.go`, add after the `GameServerConfig` block:

```go
// WebConfig holds HTTP web server settings.
type WebConfig struct {
	// Port is the TCP port for the web HTTP server. Default: 8080.
	Port int `mapstructure:"port"`
	// JWTSecret is the HS256 signing secret for JWT tokens. Must not be empty in production.
	JWTSecret string `mapstructure:"jwt_secret"`
}

// Validate checks WebConfig invariants.
//
// Postcondition: Returns nil if valid, or an error describing all violations.
func (w WebConfig) Validate() error {
	var errs []string
	if w.Port != 0 && (w.Port < 1 || w.Port > 65535) {
		errs = append(errs, fmt.Sprintf("web.port must be 1-65535 or 0 (default), got %d", w.Port))
	}
	if w.JWTSecret == "" {
		errs = append(errs, "web.jwt_secret must not be empty")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
```

Add `Web WebConfig` field to the top-level `Config` struct:
```go
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Telnet     TelnetConfig     `mapstructure:"telnet"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	GameServer GameServerConfig `mapstructure:"gameserver"`
	Web        WebConfig        `mapstructure:"web"`
}
```

Add `validateWeb` call in `Config.Validate()`:
```go
if err := validateWeb(c.Web); err != nil {
    errs = append(errs, err.Error())
}
```

Add the private validator:
```go
func validateWeb(w WebConfig) error {
	return w.Validate()
}
```

Add defaults in `setDefaults`:
```go
v.SetDefault("web.port", 8080)
```

Update `configs/dev.yaml` — add at the end:
```yaml
web:
  port: 8080
  jwt_secret: dev-secret-change-in-prod
```

**Note:** `Config.Validate()` calls `validateWeb` unconditionally. Because the telnet frontend does not set `web.jwt_secret`, the existing frontend tests will fail if they use a config without that key. The `Load` / `LoadFromViper` path will pick up the default empty string and fail. To avoid breaking the telnet frontend, `validateWeb` MUST only error on empty `JWTSecret` when `Port != 0` (i.e. when the web server is explicitly configured). Revise `WebConfig.Validate()`:

```go
func (w WebConfig) Validate() error {
	var errs []string
	if w.Port != 0 && (w.Port < 1 || w.Port > 65535) {
		errs = append(errs, fmt.Sprintf("web.port must be 1-65535 or 0 (disabled), got %d", w.Port))
	}
	// JWTSecret is only required when the web server is enabled (Port > 0).
	if w.Port > 0 && w.JWTSecret == "" {
		errs = append(errs, "web.jwt_secret must not be empty when web server is enabled")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
```

Update the test cases accordingly (port 0 + empty secret = no error; port 8080 + empty secret = error).

### Step 1.3 — Run tests

```
mise exec -- go test ./internal/config/... -v -run TestWebConfig_Validate
mise exec -- go test ./internal/config/... -v
```

Both must pass before proceeding.

---

## Task 2: Create cmd/webclient skeleton (main.go, server.go)

**REQ-WC-1, REQ-WC-2, REQ-WC-5, REQ-WC-48**

### Step 2.1 — Create cmd/webclient/server.go

```go
// Package main provides the web HTTP server for the MUD browser client.
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// Server is the web HTTP server.
//
// Invariant: grpcConn is non-nil after New returns without error.
type Server struct {
	cfg         config.WebConfig
	httpServer  *http.Server
	grpcConn    *grpc.ClientConn
	accountRepo *postgres.AccountRepository
	logger      *zap.Logger
}

// New constructs a Server, establishes the gRPC connection, and registers routes.
//
// Precondition: cfg.Port must be > 0; cfg.JWTSecret must be non-empty.
// Precondition: gameserverAddr must be a valid "host:port" string.
// Postcondition: Returns a ready-to-serve Server or a non-nil error.
func New(
	cfg config.WebConfig,
	gameserverAddr string,
	accountRepo *postgres.AccountRepository,
	logger *zap.Logger,
) (*Server, error) {
	conn, err := grpc.NewClient(gameserverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dialing gameserver: %w", err)
	}

	s := &Server{
		cfg:         cfg,
		grpcConn:    conn,
		accountRepo: accountRepo,
		logger:      logger,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	port := cfg.Port
	if port == 0 {
		port = 8080
	}
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s, nil
}

// ListenAndServe starts the HTTP server. It blocks until the server is stopped.
//
// Postcondition: Returns http.ErrServerClosed on graceful shutdown.
func (s *Server) ListenAndServe() error {
	s.logger.Info("web server listening", zap.String("addr", s.httpServer.Addr))
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server within the given deadline.
//
// Postcondition: In-flight requests are drained; gRPC connection is closed.
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	if err := s.grpcConn.Close(); err != nil {
		return fmt.Errorf("grpc close: %w", err)
	}
	return nil
}
```

### Step 2.2 — Create cmd/webclient/main.go

```go
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/observability"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

func main() {
	start := time.Now()

	configPath := flag.String("config", "configs/dev.yaml", "path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	logger, err := observability.NewLogger(cfg.Logging)
	if err != nil {
		log.Fatalf("initializing logger: %v", err)
	}
	defer logger.Sync()

	pool, err := postgres.NewPool(context.Background(), cfg.Database)
	if err != nil {
		logger.Fatal("connecting to database", zap.Error(err))
	}
	defer pool.Close()

	accountRepo := postgres.NewAccountRepository(pool)

	srv, err := New(cfg.Web, cfg.GameServer.Addr(), accountRepo, logger)
	if err != nil {
		logger.Fatal("initializing web server", zap.Error(err))
	}

	logger.Info("webclient starting",
		zap.Duration("startup", time.Since(start)),
		zap.String("gameserver_addr", cfg.GameServer.Addr()),
	)

	// Start server in background goroutine.
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for OS signal or server error.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Info("received signal, shutting down", zap.String("signal", sig.String()))
	case err := <-errCh:
		if err != nil {
			logger.Error("server error", zap.Error(err))
		}
		return
	}

	// Graceful shutdown with 10-second drain timeout (REQ-WC-48).
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
	logger.Info("web server stopped")
}
```

### Step 2.3 — Verify it compiles (no tests yet for main.go)

```
mise exec -- go build ./cmd/webclient/...
```

---

## Task 3: JWT middleware

**REQ-WC-9, REQ-WC-10**

### Step 3.1 — Write failing test

Create `cmd/webclient/middleware/jwt_test.go`:

```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/cory-johannsen/mud/cmd/webclient/middleware"
)

const testSecret = "test-secret-value"

func makeToken(secret string, accountID int64, role string, exp time.Time) string {
	claims := jwt.MapClaims{
		"account_id": accountID,
		"role":       role,
		"exp":        exp.Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString([]byte(secret))
	return signed
}

func TestRequireJWT(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.RequireJWT(testSecret, inner)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{
			name:       "valid token",
			authHeader: "Bearer " + makeToken(testSecret, 1, "player", time.Now().Add(time.Hour)),
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing header",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "malformed header",
			authHeader: "NotBearer abc",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "expired token",
			authHeader: "Bearer " + makeToken(testSecret, 1, "player", time.Now().Add(-time.Hour)),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong secret",
			authHeader: "Bearer " + makeToken("wrong-secret", 1, "player", time.Now().Add(time.Hour)),
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}
```

Run (expect failure):
```
mise exec -- go test ./cmd/webclient/middleware/... -v -run TestRequireJWT
```

### Step 3.2 — Implement

Create `cmd/webclient/middleware/jwt.go`:

```go
// Package middleware provides HTTP middleware for the web client server.
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

// ClaimsKey is the context key under which parsed JWT claims are stored.
const ClaimsKey contextKey = "jwt_claims"

// Claims holds the validated JWT payload fields.
type Claims struct {
	AccountID int64  `json:"account_id"`
	Role      string `json:"role"`
}

// RequireJWT returns an http.Handler that validates the Bearer JWT in the
// Authorization header. On success the Claims are stored in the request
// context under ClaimsKey and the next handler is called. On failure a
// JSON 401 is returned.
//
// Precondition: secret must be non-empty.
// Postcondition: next is only called when the token is valid and unexpired.
func RequireJWT(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeUnauthorized(w, "missing or malformed Authorization header")
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		}, jwt.WithExpirationRequired())
		if err != nil || !token.Valid {
			writeUnauthorized(w, "invalid or expired token")
			return
		}

		mapClaims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			writeUnauthorized(w, "malformed claims")
			return
		}

		// Extract account_id (JSON numbers unmarshal as float64).
		accountIDFloat, _ := mapClaims["account_id"].(float64)
		role, _ := mapClaims["role"].(string)

		claims := Claims{
			AccountID: int64(accountIDFloat),
			Role:      role,
		}

		ctx := context.WithValue(r.Context(), ClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromContext retrieves Claims from the context.
//
// Postcondition: ok is false if no claims are present.
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(ClaimsKey).(Claims)
	return c, ok
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

### Step 3.3 — Run tests

```
mise exec -- go test ./cmd/webclient/middleware/... -v -run TestRequireJWT
```

---

## Task 4: Auth handlers (login, register, me)

**REQ-WC-7, REQ-WC-8, REQ-WC-11**

### Step 4.1 — Write failing tests

Create `cmd/webclient/handlers/auth_test.go`:

```go
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
	return f.createFn(ctx, username, password)
}
func (f *fakeAccountStore) Authenticate(ctx context.Context, username, password string) (postgres.Account, error) {
	return f.authenticateFn(ctx, username, password)
}
func (f *fakeAccountStore) GetByID(ctx context.Context, id int64) (postgres.Account, error) {
	return f.getByIDFn(ctx, id)
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
				authenticateFn: func(ctx context.Context, u, p string) (postgres.Account, error) {
					if tt.storeFn != nil {
						return tt.storeFn(ctx, u, p)
					}
					return postgres.Account{}, nil
				},
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
				createFn: func(ctx context.Context, u, p string) (postgres.Account, error) {
					if tt.storeFn != nil {
						return tt.storeFn(ctx, u, p)
					}
					return postgres.Account{}, nil
				},
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
```

Run (expect failure):
```
mise exec -- go test ./cmd/webclient/handlers/... -v -run TestLogin
```

### Step 4.2 — Implement

Create `cmd/webclient/handlers/auth.go`:

```go
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
//
// Implementations: *postgres.AccountRepository satisfies this interface.
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
//
// Invariant: store and jwtSecret are non-nil/non-empty after NewAuthHandler.
type AuthHandler struct {
	store     AccountStore
	jwtSecret string
}

// NewAuthHandler creates an AuthHandler.
//
// Precondition: store must be non-nil; jwtSecret must be non-empty.
func NewAuthHandler(store AccountStore, jwtSecret string) *AuthHandler {
	return &AuthHandler{store: store, jwtSecret: jwtSecret}
}

// Login handles POST /api/auth/login.
//
// Precondition: request body must be valid JSON with "username" and "password" fields.
// Postcondition: Returns 200 + TokenResponse on success, 400 on bad body, 401 on invalid credentials.
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
//
// Precondition: request body must be valid JSON with "username" and "password" fields.
// Postcondition: Returns 201 + TokenResponse on success, 400 on validation failure, 409 on duplicate username.
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
//
// Precondition: JWT middleware must have placed Claims in the request context.
// Postcondition: Returns 200 + account JSON on success, 401 if claims absent.
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

// writeTokenResponse issues a JWT and writes a TokenResponse.
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

// issueJWT creates a signed HS256 JWT for the given account (24h TTL).
//
// Postcondition: Returns a signed token string or a non-nil error.
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
```

**Note:** `postgres.AccountRepository` does not expose a `GetByID` method. Add it to `internal/storage/postgres/account.go`:

```go
// GetByID retrieves an account by its primary key.
//
// Precondition: id must be > 0.
// Postcondition: Returns the Account or ErrAccountNotFound.
func (r *AccountRepository) GetByID(ctx context.Context, id int64) (Account, error) {
	var acct Account
	err := r.db.QueryRow(ctx,
		`SELECT id, username, password_hash, role, created_at
		 FROM accounts WHERE id = $1`,
		id,
	).Scan(&acct.ID, &acct.Username, &acct.PasswordHash, &acct.Role, &acct.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Account{}, ErrAccountNotFound
		}
		return Account{}, fmt.Errorf("querying account by id: %w", err)
	}
	return acct, nil
}
```

### Step 4.3 — Wire routes in server.go

Add `registerRoutes` method to `server.go`:

```go
// registerRoutes wires all HTTP routes onto mux.
//
// Postcondition: /api/auth/* routes are registered; JWT middleware protects all
// other /api/* routes.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	authHandler := handlers.NewAuthHandler(s.accountRepo, s.cfg.JWTSecret)

	// Public auth routes — no JWT required.
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/register", authHandler.Register)

	// Protected auth routes.
	mux.Handle("GET /api/auth/me",
		middleware.RequireJWT(s.cfg.JWTSecret, http.HandlerFunc(authHandler.Me)))

	// Static files + SPA fallback (Task 5).
	mux.HandleFunc("/", s.serveIndex)
}
```

Add required imports to `server.go`:
```go
"github.com/cory-johannsen/mud/cmd/webclient/handlers"
"github.com/cory-johannsen/mud/cmd/webclient/middleware"
```

### Step 4.4 — Run tests

```
mise exec -- go test ./cmd/webclient/handlers/... -v
mise exec -- go test ./cmd/webclient/... -v
mise exec -- go build ./cmd/webclient/...
```

---

## Task 5: Static file serving + SPA routing

**REQ-WC-3, REQ-WC-4**

### Step 5.1 — Write failing test

Create `cmd/webclient/server_static_test.go`:

```go
package main_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestSPAFallback verifies that non-API routes serve index.html.
func TestSPAFallback(t *testing.T) {
	// Write a temp index.html.
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html>SPA</html>"), 0644); err != nil {
		t.Fatal(err)
	}

	// Point WEB_STATIC_DIR to the temp dir.
	t.Setenv("WEB_STATIC_DIR", dir)

	handler := buildStaticHandler(dir)

	tests := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{"/", http.StatusOK, "<html>SPA"},
		{"/characters", http.StatusOK, "<html>SPA"},
		{"/game", http.StatusOK, "<html>SPA"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
			if body := rr.Body.String(); len(body) == 0 {
				t.Error("expected non-empty body")
			}
		})
	}
}
```

Run (expect failure):
```
mise exec -- go test ./cmd/webclient/... -v -run TestSPAFallback
```

### Step 5.2 — Implement static serving in server.go

Add the following to `server.go`. The embed directive requires `ui/dist` to exist at build time; the test uses `WEB_STATIC_DIR` to bypass the embed.

```go
import (
	"embed"
	"io/fs"
	"os"
)

//go:embed ui/dist
var embeddedUI embed.FS

// buildStaticHandler constructs an http.Handler for static files.
// If WEB_STATIC_DIR is set, files are served from the filesystem.
// Otherwise the embedded ui/dist is used.
//
// Postcondition: Returns a handler that serves index.html for all unmatched paths.
func buildStaticHandler(staticDir string) http.Handler {
	var fsys fs.FS
	if staticDir != "" {
		fsys = os.DirFS(staticDir)
	} else {
		sub, err := fs.Sub(embeddedUI, "ui/dist")
		if err != nil {
			panic("embedded ui/dist missing: " + err.Error())
		}
		fsys = sub
	}
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to open the requested file; fall back to index.html.
		f, err := fsys.Open(r.URL.Path[1:]) // strip leading "/"
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html for all non-file paths.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
```

Update `serveIndex` on the Server to delegate to `buildStaticHandler`:

```go
// serveIndex is registered on "/" and handles SPA routing + static assets.
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	staticDir := os.Getenv("WEB_STATIC_DIR")
	buildStaticHandler(staticDir).ServeHTTP(w, r)
}
```

Export `buildStaticHandler` as a package-level function so the test can call it directly (keep it unexported and test via the package `main_test` helper, using `//go:linkname` is not needed — move the helper into a `static.go` file and keep it exported for testability, or make the test a whitebox test in `package main`).

**Revised approach:** Move `buildStaticHandler` to `cmd/webclient/static.go` (same `package main`) so `server_static_test.go` in `package main` can call it directly as a whitebox test.

### Step 5.3 — Create placeholder ui/dist/index.html

The `//go:embed ui/dist` directive requires at least one file to exist at build time:

```
mkdir -p cmd/webclient/ui/dist
echo '<!DOCTYPE html><html><body>Loading...</body></html>' > cmd/webclient/ui/dist/index.html
```

Add to `.gitignore`:
```
cmd/webclient/ui/node_modules/
cmd/webclient/ui/dist/
```

**Note:** Because `ui/dist/` is in `.gitignore`, the embed will fail in a clean checkout without running `make ui-build` first. This is intentional and documented in the Phase 2 plan (React scaffolding). For Phase 1, commit the placeholder `index.html` with a `!cmd/webclient/ui/dist/index.html` gitignore exception to unblock the Go build.

### Step 5.4 — Run full test suite

```
mise exec -- go test ./cmd/webclient/... -v
mise exec -- go test ./internal/config/... -v
mise exec -- go build ./cmd/webclient/...
mise exec -- go build ./cmd/frontend/...
mise exec -- go vet ./...
```

All tests MUST pass before this phase is considered complete.

---

## Commit Checklist

- [ ] `internal/config/config.go` — WebConfig added, validateWeb wired, setDefaults updated
- [ ] `configs/dev.yaml` — `web:` section added
- [ ] `internal/storage/postgres/account.go` — GetByID method added
- [ ] `cmd/webclient/main.go` — entry point with graceful shutdown
- [ ] `cmd/webclient/server.go` — Server struct, New, routes, Shutdown
- [ ] `cmd/webclient/static.go` — buildStaticHandler, embed directive
- [ ] `cmd/webclient/middleware/jwt.go` — RequireJWT, ClaimsFromContext
- [ ] `cmd/webclient/handlers/auth.go` — AuthHandler, AccountStore interface
- [ ] `cmd/webclient/ui/dist/index.html` — placeholder (gitignore exception)
- [ ] `.gitignore` — ui/dist and node_modules excluded (placeholder excepted)
- [ ] All tests pass: `mise exec -- go test ./... -count=1`
