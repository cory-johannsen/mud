// internal/client/auth/auth.go
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// ErrUnauthorized is returned when the server responds with HTTP 401.
var ErrUnauthorized = errors.New("unauthorized")

// ErrNameTaken is returned when the server responds with HTTP 409 on character creation.
var ErrNameTaken = errors.New("character name already taken")

// ErrValidation is returned when the server responds with HTTP 400.
type ErrValidation struct{ Message string }

func (e ErrValidation) Error() string { return "validation error: " + e.Message }

// ErrNetwork is returned when the HTTP request itself fails (connection refused, timeout, etc.).
type ErrNetwork struct{ Cause error }

func (e ErrNetwork) Error() string { return "network error: " + e.Cause.Error() }
func (e ErrNetwork) Unwrap() error { return e.Cause }

// Account holds the authenticated account details returned by /api/auth/me.
type Account struct {
	ID       int64
	Username string
	Role     string
}

// CharacterSummary is a character as returned by the character list and creation endpoints.
type CharacterSummary struct {
	ID        int64
	Name      string
	Job       string
	Level     int
	CurrentHP int
	MaxHP     int
	Region    string
	Archetype string
}

// CreateCharacterRequest is the payload for POST /api/characters.
type CreateCharacterRequest struct {
	Name      string
	Job       string
	Archetype string
	Region    string
	Gender    string
}

// CharacterOptions contains the available choices for character creation.
type CharacterOptions struct {
	Regions    []string
	Jobs       []string
	Archetypes map[string][]string // job → available archetypes
}

// Client is an HTTP client for the webclient REST API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New creates a Client targeting the given base URL (e.g. "http://localhost:8080").
func New(baseURL string) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{}}
}

// Login authenticates with username/password and returns a JWT on success.
func (c *Client) Login(ctx context.Context, username, password string) (string, error) {
	body := map[string]string{"username": username, "password": password}
	var resp struct {
		Token string `json:"token"`
	}
	if err := c.post(ctx, "/api/auth/login", "", body, &resp); err != nil {
		return "", err
	}
	return resp.Token, nil
}

// Register creates an account and returns a JWT on success.
func (c *Client) Register(ctx context.Context, username, password string) (string, error) {
	body := map[string]string{"username": username, "password": password}
	var resp struct {
		Token string `json:"token"`
	}
	if err := c.post(ctx, "/api/auth/register", "", body, &resp); err != nil {
		return "", err
	}
	return resp.Token, nil
}

// Me returns the authenticated account details for the given JWT.
func (c *Client) Me(ctx context.Context, jwt string) (*Account, error) {
	var resp struct {
		AccountID int64  `json:"account_id"`
		Username  string `json:"username"`
		Role      string `json:"role"`
	}
	if err := c.get(ctx, "/api/auth/me", jwt, &resp); err != nil {
		return nil, err
	}
	return &Account{ID: resp.AccountID, Username: resp.Username, Role: resp.Role}, nil
}

// ListCharacters returns the characters belonging to the authenticated account.
func (c *Client) ListCharacters(ctx context.Context, jwt string) ([]CharacterSummary, error) {
	var raw []struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Job       string `json:"job"`
		Level     int    `json:"level"`
		CurrentHP int    `json:"current_hp"`
		MaxHP     int    `json:"max_hp"`
		Region    string `json:"region"`
		Archetype string `json:"archetype"`
	}
	if err := c.get(ctx, "/api/characters", jwt, &raw); err != nil {
		return nil, err
	}
	out := make([]CharacterSummary, len(raw))
	for i, r := range raw {
		out[i] = CharacterSummary{
			ID: r.ID, Name: r.Name, Job: r.Job, Level: r.Level,
			CurrentHP: r.CurrentHP, MaxHP: r.MaxHP, Region: r.Region, Archetype: r.Archetype,
		}
	}
	return out, nil
}

// CreateCharacter creates a new character and returns its summary.
func (c *Client) CreateCharacter(ctx context.Context, jwt string, req CreateCharacterRequest) (*CharacterSummary, error) {
	body := map[string]string{
		"name": req.Name, "job": req.Job, "archetype": req.Archetype,
		"region": req.Region, "gender": req.Gender,
	}
	var resp struct {
		Character struct {
			ID        int64  `json:"id"`
			Name      string `json:"name"`
			Job       string `json:"job"`
			Level     int    `json:"level"`
			CurrentHP int    `json:"current_hp"`
			MaxHP     int    `json:"max_hp"`
			Region    string `json:"region"`
			Archetype string `json:"archetype"`
		} `json:"character"`
	}
	if err := c.post(ctx, "/api/characters", jwt, body, &resp); err != nil {
		return nil, err
	}
	r := resp.Character
	return &CharacterSummary{
		ID: r.ID, Name: r.Name, Job: r.Job, Level: r.Level,
		CurrentHP: r.CurrentHP, MaxHP: r.MaxHP, Region: r.Region, Archetype: r.Archetype,
	}, nil
}

// CheckName returns true if the character name is available.
func (c *Client) CheckName(ctx context.Context, name string) (bool, error) {
	u := "/api/characters/check-name?name=" + url.QueryEscape(name)
	var resp struct {
		Available bool `json:"available"`
	}
	if err := c.get(ctx, u, "", &resp); err != nil {
		return false, err
	}
	return resp.Available, nil
}

// CharacterOptions returns available regions, jobs, and archetypes for character creation.
func (c *Client) CharacterOptions(ctx context.Context, jwt string) (*CharacterOptions, error) {
	var resp struct {
		Regions    []string            `json:"regions"`
		Jobs       []string            `json:"jobs"`
		Archetypes map[string][]string `json:"archetypes"`
	}
	if err := c.get(ctx, "/api/characters/options", jwt, &resp); err != nil {
		return nil, err
	}
	return &CharacterOptions{Regions: resp.Regions, Jobs: resp.Jobs, Archetypes: resp.Archetypes}, nil
}

// --- internal helpers ---

func (c *Client) get(ctx context.Context, path, jwt string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return ErrNetwork{Cause: err}
	}
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}
	return c.do(req, out)
}

func (c *Client) post(ctx context.Context, path, jwt string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return ErrNetwork{Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return ErrNetwork{Cause: err}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrNetwork{Cause: err}
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		if out != nil {
			return json.Unmarshal(data, out)
		}
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusConflict:
		return ErrNameTaken
	case http.StatusBadRequest:
		var e struct{ Error string `json:"error"` }
		_ = json.Unmarshal(data, &e)
		return ErrValidation{Message: e.Error}
	default:
		var e struct{ Error string `json:"error"` }
		_ = json.Unmarshal(data, &e)
		return fmt.Errorf("server error %d: %s", resp.StatusCode, e.Error)
	}
}
