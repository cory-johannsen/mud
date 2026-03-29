package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	postgres "github.com/cory-johannsen/mud/internal/storage/postgres"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

// CharacterLister returns all characters for an account.
type CharacterLister interface {
	ListByAccount(ctx context.Context, accountID int64) ([]*character.Character, error)
}

// CharacterCreator creates a new character and returns it.
type CharacterCreator interface {
	Create(ctx context.Context, c *character.Character) (*character.Character, error)
}

// NameChecker reports whether a character name is available.
type NameChecker interface {
	IsNameAvailable(ctx context.Context, name string) (bool, error)
}

// CharacterOptions holds ruleset data loaded at startup for the creation wizard.
type CharacterOptions struct {
	Regions    []*ruleset.Region
	Jobs       []*ruleset.Job
	Archetypes []*ruleset.Archetype
}

// CharacterResponse is the JSON shape returned for a single character.
type CharacterResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Job       string `json:"job"`
	Level     int    `json:"level"`
	CurrentHP int32  `json:"current_hp"`
	MaxHP     int32  `json:"max_hp"`
	Region    string `json:"region"`
	Archetype string `json:"archetype"`
}

// CharacterHandler serves all /api/characters endpoints.
type CharacterHandler struct {
	lister    CharacterLister
	creator   CharacterCreator
	checker   NameChecker
	getter    CharacterGetter
	options   *CharacterOptions
	jwtSecret string
}

// NewCharacterHandler creates a CharacterHandler.
func NewCharacterHandler(lister CharacterLister, creator CharacterCreator, checker NameChecker) *CharacterHandler {
	return &CharacterHandler{lister: lister, creator: creator, checker: checker}
}

// WithJWTSecret attaches the JWT secret so HandlePlay can issue character-scoped tokens.
func (h *CharacterHandler) WithJWTSecret(secret string) *CharacterHandler {
	h.jwtSecret = secret
	return h
}

// WithGetter attaches a CharacterGetter for ownership verification in HandlePlay.
func (h *CharacterHandler) WithGetter(g CharacterGetter) *CharacterHandler {
	h.getter = g
	return h
}

// WithOptions sets the ruleset options for the creation wizard endpoints.
func (h *CharacterHandler) WithOptions(opts *CharacterOptions) *CharacterHandler {
	h.options = opts
	return h
}

// ListCharacters handles GET /api/characters.
//
// Precondition: Request context MUST carry account_id via WithAccountID.
// Postcondition: Writes JSON array of CharacterResponse; HTTP 500 on store error.
func (h *CharacterHandler) ListCharacters(w http.ResponseWriter, r *http.Request) {
	accountID := AccountIDFromContext(r.Context())
	chars, err := h.lister.ListByAccount(r.Context(), accountID)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	resp := make([]CharacterResponse, 0, len(chars))
	for _, c := range chars {
		resp = append(resp, characterToResponse(c))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// characterToResponse maps a Character domain object to its API response shape.
func characterToResponse(c *character.Character) CharacterResponse {
	return CharacterResponse{
		ID:        c.ID,
		Name:      c.Name,
		Job:       c.Class,
		Level:     c.Level,
		CurrentHP: int32(c.CurrentHP),
		MaxHP:     int32(c.MaxHP),
		Region:    c.Region,
		Archetype: c.Team,
	}
}

// createCharacterRequest is the JSON payload for POST /api/characters.
type createCharacterRequest struct {
	Name      string `json:"name"`
	Job       string `json:"job"`
	Archetype string `json:"archetype"`
	Region    string `json:"region"`
	Gender    string `json:"gender"`
}

func (r createCharacterRequest) validate() string {
	n := strings.TrimSpace(r.Name)
	if len(n) < 3 || len(n) > 20 {
		return "name must be 3-20 characters"
	}
	if r.Job == "" {
		return "job is required"
	}
	if r.Archetype == "" {
		return "archetype is required"
	}
	if r.Region == "" {
		return "region is required"
	}
	if r.Gender == "" {
		return "gender is required"
	}
	return ""
}

// CreateCharacter handles POST /api/characters.
//
// Precondition: Request context MUST carry account_id; creator MUST be non-nil.
// Postcondition: Returns HTTP 201 with {"character": CharacterResponse} on success,
//
//	HTTP 400 on validation failure, HTTP 409 if name taken, HTTP 500 on store error.
func (h *CharacterHandler) CreateCharacter(w http.ResponseWriter, r *http.Request) {
	var req createCharacterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if msg := req.validate(); msg != "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
		return
	}
	accountID := AccountIDFromContext(r.Context())
	c := &character.Character{
		AccountID: accountID,
		Name:      strings.TrimSpace(req.Name),
		Class:     req.Job,
		Team:      req.Archetype,
		Region:    req.Region,
		Gender:    req.Gender,
		Level:     1,
	}
	created, err := h.creator.Create(r.Context(), c)
	if err != nil {
		if errors.Is(err, postgres.ErrCharacterNameTaken) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "name already taken"})
			return
		}
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"character": characterToResponse(created)})
}

type regionResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type jobResponse struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	Archetype         string `json:"archetype"`
	KeyAbility        string `json:"key_ability"`
	HitPointsPerLevel int    `json:"hit_points_per_level"`
}

type archetypeResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListOptions handles GET /api/characters/options.
//
// Precondition: h.options MUST be non-nil (set via WithOptions at startup).
// Postcondition: Returns JSON with regions, jobs, and archetypes arrays.
func (h *CharacterHandler) ListOptions(w http.ResponseWriter, r *http.Request) {
	if h.options == nil {
		http.Error(w, `{"error":"options not loaded"}`, http.StatusInternalServerError)
		return
	}
	regions := make([]regionResponse, 0, len(h.options.Regions))
	for _, reg := range h.options.Regions {
		regions = append(regions, regionResponse{
			ID:          reg.ID,
			Name:        reg.Name,
			Description: reg.Description,
		})
	}
	jobs := make([]jobResponse, 0, len(h.options.Jobs))
	for _, job := range h.options.Jobs {
		jobs = append(jobs, jobResponse{
			ID:                job.ID,
			Name:              job.Name,
			Description:       job.Description,
			Archetype:         job.Archetype,
			KeyAbility:        job.KeyAbility,
			HitPointsPerLevel: job.HitPointsPerLevel,
		})
	}
	archetypes := make([]archetypeResponse, 0, len(h.options.Archetypes))
	for _, arch := range h.options.Archetypes {
		archetypes = append(archetypes, archetypeResponse{
			ID:          arch.ID,
			Name:        arch.Name,
			Description: arch.Description,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"regions":    regions,
		"jobs":       jobs,
		"archetypes": archetypes,
	})
}

// HandlePlay handles POST /api/characters/{id}/play.
//
// Precondition: h.getter and h.jwtSecret MUST be set (via WithGetter and WithJWTSecret).
// Postcondition: Returns {"token": <JWT with character_id>}; HTTP 403 if character not owned by caller.
func (h *CharacterHandler) HandlePlay(w http.ResponseWriter, r *http.Request) {
	if h.getter == nil || h.jwtSecret == "" {
		http.Error(w, `{"error":"play not configured"}`, http.StatusInternalServerError)
		return
	}
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, `{"error":"missing id"}`, http.StatusBadRequest)
		return
	}
	var charID int64
	if _, err := fmt.Sscan(idStr, &charID); err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	accountID := AccountIDFromContext(r.Context())
	char, err := h.getter.GetByID(r.Context(), charID)
	if err != nil {
		http.Error(w, `{"error":"character not found"}`, http.StatusNotFound)
		return
	}
	if char.AccountID != accountID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	claims := jwt.MapClaims{
		"account_id":   accountID,
		"character_id": charID,
		"role":         RoleFromContext(r.Context()),
		"exp":          time.Now().Add(24 * time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := tok.SignedString([]byte(h.jwtSecret))
	if err != nil {
		http.Error(w, `{"error":"token generation failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// CheckName handles GET /api/characters/check-name?name=<value>.
//
// Precondition: h.checker MUST be non-nil.
// Postcondition: Returns {"available": bool}; HTTP 400 if name query param is absent.
func (h *CharacterHandler) CheckName(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if strings.TrimSpace(name) == "" {
		http.Error(w, `{"error":"name query parameter is required"}`, http.StatusBadRequest)
		return
	}
	available, err := h.checker.IsNameAvailable(r.Context(), name)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"available": available})
}
