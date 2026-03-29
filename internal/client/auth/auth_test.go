// internal/client/auth/auth_test.go
package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cory-johannsen/mud/internal/client/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Login_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/auth/login", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"token":      "test.jwt.token",
			"account_id": 42,
			"role":       "player",
		})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	jwt, err := c.Login(context.Background(), "alice", "password")
	require.NoError(t, err)
	assert.Equal(t, "test.jwt.token", jwt)
}

func TestClient_Login_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	_, err := c.Login(context.Background(), "alice", "wrong")
	require.ErrorIs(t, err, auth.ErrUnauthorized)
}

func TestClient_Register_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/auth/register", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": "new.jwt.token"})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	jwt, err := c.Register(context.Background(), "newuser", "pass")
	require.NoError(t, err)
	assert.Equal(t, "new.jwt.token", jwt)
}

func TestClient_Register_ValidationError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "username too short"})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	_, err := c.Register(context.Background(), "x", "pass")
	var ve auth.ErrValidation
	require.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Message, "username too short")
}

func TestClient_Me_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/auth/me", r.URL.Path)
		assert.Equal(t, "Bearer mytoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"account_id": 99,
			"username":   "alice",
			"role":       "admin",
		})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	acc, err := c.Me(context.Background(), "mytoken")
	require.NoError(t, err)
	assert.Equal(t, int64(99), acc.ID)
	assert.Equal(t, "alice", acc.Username)
	assert.Equal(t, "admin", acc.Role)
}

func TestClient_ListCharacters_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/characters", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "name": "Zara", "job": "hunter", "level": 3,
				"current_hp": 25, "max_hp": 30, "region": "vantucky", "archetype": "sniper"},
		})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	chars, err := c.ListCharacters(context.Background(), "jwt")
	require.NoError(t, err)
	require.Len(t, chars, 1)
	assert.Equal(t, "Zara", chars[0].Name)
	assert.Equal(t, 3, chars[0].Level)
}

func TestClient_CreateCharacter_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/characters", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"character": map[string]any{
				"id": 42, "name": "Zara", "job": "hunter", "level": 1,
				"current_hp": 20, "max_hp": 20, "region": "vantucky", "archetype": "sniper",
			},
		})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	ch, err := c.CreateCharacter(context.Background(), "jwt", auth.CreateCharacterRequest{
		Name: "Zara", Job: "hunter", Archetype: "sniper", Region: "vantucky", Gender: "female",
	})
	require.NoError(t, err)
	assert.Equal(t, "Zara", ch.Name)
	assert.Equal(t, int64(42), ch.ID)
}

func TestClient_CreateCharacter_NameTaken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "name taken"})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	_, err := c.CreateCharacter(context.Background(), "jwt", auth.CreateCharacterRequest{Name: "Zara"})
	require.ErrorIs(t, err, auth.ErrNameTaken)
}

func TestClient_CheckName_Available(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Zara", r.URL.Query().Get("name"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"available": true})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	ok, err := c.CheckName(context.Background(), "Zara")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestClient_CharacterOptions_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/characters/options", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"regions": []string{"vantucky", "portland"},
			"jobs":    []string{"hunter", "fixer"},
			"archetypes": map[string][]string{
				"hunter": {"sniper", "trapper"},
				"fixer":  {"face", "tech"},
			},
		})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	opts, err := c.CharacterOptions(context.Background(), "jwt")
	require.NoError(t, err)
	assert.Equal(t, []string{"vantucky", "portland"}, opts.Regions)
	assert.Equal(t, []string{"sniper", "trapper"}, opts.Archetypes["hunter"])
}

func TestClient_NetworkError(t *testing.T) {
	c := auth.New("http://localhost:1") // nothing listening
	_, err := c.Login(context.Background(), "x", "y")
	var ne auth.ErrNetwork
	require.ErrorAs(t, err, &ne)
}
