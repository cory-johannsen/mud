// Package middleware_test tests the admin role enforcement middleware.
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/cmd/webclient/middleware"
)


// okHandler is a sentinel that records whether it was called.
func okHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

// TestRequireAdminRole_AdminPasses verifies role "admin" reaches the next handler.
func TestRequireAdminRole_AdminPasses(t *testing.T) {
	called := false
	chain := middleware.RequireJWT(testSecret, middleware.RequireAdminRole(okHandler(&called)))

	tok := makeToken(testSecret, 1, "admin", time.Now().Add(time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/api/admin/players", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if !called {
		t.Fatal("handler should have been called for role admin")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestRequireAdminRole_ModeratorPasses verifies role "moderator" reaches the next handler.
func TestRequireAdminRole_ModeratorPasses(t *testing.T) {
	called := false
	chain := middleware.RequireJWT(testSecret, middleware.RequireAdminRole(okHandler(&called)))

	tok := makeToken(testSecret, 2, "moderator", time.Now().Add(time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/api/admin/players", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if !called {
		t.Fatal("handler should have been called for role moderator")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestRequireAdminRole_PlayerForbidden verifies role "player" receives 403.
func TestRequireAdminRole_PlayerForbidden(t *testing.T) {
	called := false
	chain := middleware.RequireJWT(testSecret, middleware.RequireAdminRole(okHandler(&called)))

	tok := makeToken(testSecret, 3, "player", time.Now().Add(time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/api/admin/players", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if called {
		t.Fatal("handler must NOT be called for role player")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// TestRequireAdminRole_NoJWTUnauthorized verifies missing JWT returns 401.
func TestRequireAdminRole_NoJWTUnauthorized(t *testing.T) {
	called := false
	// Wrap ONLY in RequireAdminRole (no JWT); claims absent.
	handler := middleware.RequireAdminRole(okHandler(&called))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/players", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if called {
		t.Fatal("handler must NOT be called without JWT claims")
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestRequireAdminRole_Property is a property test over all roles.
func TestRequireAdminRole_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		role := rapid.StringMatching(`^[a-z]{3,12}$`).Draw(rt, "role")

		called := false
		chain := middleware.RequireJWT(testSecret, middleware.RequireAdminRole(okHandler(&called)))

		tok := makeToken(testSecret, 99, role, time.Now().Add(time.Hour))
		req := httptest.NewRequest(http.MethodGet, "/api/admin/x", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		chain.ServeHTTP(w, req)

		allowed := role == "admin" || role == "moderator"
		if allowed && !called {
			rt.Fatalf("role %q should pass but handler was not called", role)
		}
		if !allowed && called {
			rt.Fatalf("role %q should be forbidden but handler was called", role)
		}
		if allowed && w.Code != http.StatusOK {
			rt.Fatalf("role %q: expected 200, got %d", role, w.Code)
		}
		if !allowed && w.Code != http.StatusForbidden {
			rt.Fatalf("role %q: expected 403, got %d", role, w.Code)
		}
	})
}
