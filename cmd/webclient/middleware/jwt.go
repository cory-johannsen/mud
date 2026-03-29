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
// Authorization header or the ?token= query parameter (for SSE/EventSource clients).
// On success the Claims are stored in the request context under ClaimsKey and the
// next handler is called. On failure a JSON 401 is returned.
//
// Precondition: secret must be non-empty.
// Postcondition: next is only called when the token is valid and unexpired.
func RequireJWT(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenStr string
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			tokenStr = strings.TrimPrefix(auth, "Bearer ")
		} else if q := r.URL.Query().Get("token"); q != "" {
			tokenStr = q
		}
		if tokenStr == "" {
			writeUnauthorized(w, "missing or malformed Authorization header")
			return
		}

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
