package middleware

import (
	"encoding/json"
	"net/http"
)

// RequireAdminRole enforces that the JWT role in context is "admin" or "moderator".
// It MUST be applied after RequireJWT so that Claims are present in the context.
//
// Precondition: RequireJWT middleware MUST have run and set Claims in context.
// Postcondition: Returns HTTP 401 if no claims present; HTTP 403 if role is insufficient;
// otherwise calls next.
func RequireAdminRole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		if claims.Role != "admin" && claims.Role != "moderator" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
