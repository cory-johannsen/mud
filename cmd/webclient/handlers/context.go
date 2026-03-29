// Package handlers provides HTTP request handlers for the web client API.
package handlers

import "context"

type contextKey int

const (
	accountIDKey  contextKey = iota
	characterIDKey
	roleKey
)

// WithAccountID stores an account ID in the context.
func WithAccountID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, accountIDKey, id)
}

// AccountIDFromContext retrieves the account ID from the context.
// Returns 0 if not set.
func AccountIDFromContext(ctx context.Context) int64 {
	v, _ := ctx.Value(accountIDKey).(int64)
	return v
}

// WithCharacterID stores a character ID in the context.
func WithCharacterID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, characterIDKey, id)
}

// CharacterIDFromContext retrieves the character ID from the context.
func CharacterIDFromContext(ctx context.Context) int64 {
	v, _ := ctx.Value(characterIDKey).(int64)
	return v
}
