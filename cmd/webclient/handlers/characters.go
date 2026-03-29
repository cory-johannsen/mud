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
	"github.com/cory-johannsen/mud/internal/game/technology"
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
	Regions      []*ruleset.Region
	Jobs         []*ruleset.Job
	Archetypes   []*ruleset.Archetype
	Teams        []*ruleset.Team
	Feats        []*ruleset.Feat       // may be nil if not loaded
	Skills       []*ruleset.Skill      // may be nil if not loaded
	TechRegistry *technology.Registry  // may be nil if not loaded
}

// AbilityBoostsAdder persists per-character ability boost choices.
//
// Precondition: characterID > 0; source and ability must be non-empty.
// Postcondition: Exactly one row exists for (character_id, source, ability).
type AbilityBoostsAdder interface {
	Add(ctx context.Context, characterID int64, source, ability string) error
}

// SkillsSetter persists the full skill proficiency map for a character.
//
// Precondition: characterID > 0; skills must not be nil.
// Postcondition: character_skills rows match skills exactly.
type SkillsSetter interface {
	SetAll(ctx context.Context, characterID int64, skills map[string]string) error
}

// FeatsSetter persists the full feat list for a character.
//
// Precondition: characterID > 0; feats must not be nil.
// Postcondition: character_feats rows match feats exactly.
type FeatsSetter interface {
	SetAll(ctx context.Context, characterID int64, feats []string) error
}

// HardwiredTechSetter persists hardwired technology assignments.
//
// Precondition: characterID > 0.
// Postcondition: Exactly the provided techIDs are stored for the character.
type HardwiredTechSetter interface {
	SetAll(ctx context.Context, characterID int64, techIDs []string) error
}

// SpontaneousTechAdder persists spontaneous technology known-slot assignments.
//
// Precondition: characterID > 0; techID must be non-empty; level > 0.
// Postcondition: A row for (character_id, tech_id) exists with the given level.
type SpontaneousTechAdder interface {
	Add(ctx context.Context, characterID int64, techID string, level int) error
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
	lister       CharacterLister
	creator      CharacterCreator
	checker      NameChecker
	getter       CharacterGetter
	options      *CharacterOptions
	jwtSecret    string
	boostsAdder  AbilityBoostsAdder    // may be nil
	skillsSetter SkillsSetter           // may be nil
	featsSetter  FeatsSetter            // may be nil
	hwTechSetter HardwiredTechSetter    // may be nil
	spontAdder   SpontaneousTechAdder   // may be nil
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

// WithPersistenceRepos attaches the persistence repositories needed to pre-populate
// character choices at creation so the gameserver skips first-login prompts.
//
// Precondition: All arguments may be nil (choices are silently skipped if repo is nil).
// Postcondition: Returns h for chaining.
func (h *CharacterHandler) WithPersistenceRepos(
	boosts AbilityBoostsAdder,
	skills SkillsSetter,
	feats FeatsSetter,
	hwTech HardwiredTechSetter,
	spont SpontaneousTechAdder,
) *CharacterHandler {
	h.boostsAdder = boosts
	h.skillsSetter = skills
	h.featsSetter = feats
	h.hwTechSetter = hwTech
	h.spontAdder = spont
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

// spontaneousChoiceRequest carries one spontaneous technology selection from the client.
type spontaneousChoiceRequest struct {
	ID    string `json:"id"`
	Level int    `json:"level"`
}

// createCharacterRequest is the JSON payload for POST /api/characters.
type createCharacterRequest struct {
	Name               string                     `json:"name"`
	Job                string                     `json:"job"`
	Team               string                     `json:"team"`
	Region             string                     `json:"region"`
	Gender             string                     `json:"gender"`
	ArchetypeBoosts    []string                   `json:"archetype_boosts"`     // player's chosen free archetype ability boosts
	RegionBoosts       []string                   `json:"region_boosts"`        // player's chosen free region ability boosts
	SkillChoices       []string                   `json:"skill_choices"`        // job skill choice selections
	FeatChoices        []string                   `json:"feat_choices"`         // job feat choice selections
	GeneralFeatChoices []string                   `json:"general_feat_choices"` // general feat selections
	SpontaneousChoices []spontaneousChoiceRequest `json:"spontaneous_choices"`  // spontaneous tech choices
}

func (r createCharacterRequest) validate() string {
	n := strings.TrimSpace(r.Name)
	if len(n) < 3 || len(n) > 20 {
		return "name must be 3-20 characters"
	}
	if r.Job == "" {
		return "job is required"
	}
	if r.Team == "" {
		return "team is required"
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
	startRoom := ""
	if h.options != nil {
		for _, t := range h.options.Teams {
			if t.ID == req.Team {
				startRoom = t.StartRoom
				break
			}
		}
	}
	c := &character.Character{
		AccountID: accountID,
		Name:      strings.TrimSpace(req.Name),
		Class:     req.Job,
		Team:      req.Team,
		Region:    req.Region,
		Gender:    req.Gender,
		Level:     1,
		Location:  startRoom,
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

	// Persist character choices (ability boosts, skills, feats, technologies).
	// Best-effort: a failure in persistence does not roll back the character creation.
	h.persistCharacterChoices(r.Context(), created.ID, req)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"character": characterToResponse(created)})
}

// persistCharacterChoices persists ability boosts, skills, feats, and technology
// choices for a newly created character. Each sub-section is independent and
// best-effort: a failure in one does not abort the others.
//
// Precondition: characterID > 0; req is the validated creation request.
// Postcondition: All available choices are persisted; any error is silently swallowed
// (the caller's response is already determined by the character creation itself).
func (h *CharacterHandler) persistCharacterChoices(ctx context.Context, characterID int64, req createCharacterRequest) {
	// Ability boosts: archetype free boosts and region free boosts.
	if h.boostsAdder != nil {
		for _, ability := range req.ArchetypeBoosts {
			_ = h.boostsAdder.Add(ctx, characterID, "archetype", ability)
		}
		for _, ability := range req.RegionBoosts {
			_ = h.boostsAdder.Add(ctx, characterID, "region", ability)
		}
	}

	// Skills: build full skill map from job grants + player choices.
	if h.skillsSetter != nil && h.options != nil && h.options.Skills != nil {
		var job *ruleset.Job
		for _, j := range h.options.Jobs {
			if j.ID == req.Job {
				job = j
				break
			}
		}
		if job != nil {
			allSkillIDs := make([]string, len(h.options.Skills))
			for i, sk := range h.options.Skills {
				allSkillIDs[i] = sk.ID
			}
			skillMap := character.BuildSkillsFromJob(job, allSkillIDs, req.SkillChoices)
			_ = h.skillsSetter.SetAll(ctx, characterID, skillMap)
		}
	}

	// Feats: build feat list from job grants + player choices.
	if h.featsSetter != nil && h.options != nil {
		var job *ruleset.Job
		for _, j := range h.options.Jobs {
			if j.ID == req.Job {
				job = j
				break
			}
		}
		if job != nil {
			feats := character.BuildFeatsFromJob(job, req.FeatChoices, req.GeneralFeatChoices, nil)
			if len(feats) > 0 {
				_ = h.featsSetter.SetAll(ctx, characterID, feats)
			}
		}
	}

	// Hardwired technologies: from job grants only (no player choice).
	if h.hwTechSetter != nil && h.options != nil {
		var job *ruleset.Job
		for _, j := range h.options.Jobs {
			if j.ID == req.Job {
				job = j
				break
			}
		}
		if job != nil && job.TechnologyGrants != nil && len(job.TechnologyGrants.Hardwired) > 0 {
			_ = h.hwTechSetter.SetAll(ctx, characterID, job.TechnologyGrants.Hardwired)
		}
	}

	// Spontaneous technologies: player choices from the job's spontaneous pool.
	if h.spontAdder != nil {
		for _, choice := range req.SpontaneousChoices {
			_ = h.spontAdder.Add(ctx, characterID, choice.ID, choice.Level)
		}
	}
}

type abilityBoostGrantResponse struct {
	Fixed []string `json:"fixed,omitempty"`
	Free  int      `json:"free,omitempty"`
}

type skillGrantsResponse struct {
	Fixed   []string             `json:"fixed,omitempty"`
	Choices *skillChoicesResponse `json:"choices,omitempty"`
}

type skillChoicesResponse struct {
	Pool  []string `json:"pool"`
	Count int      `json:"count"`
}

type featGrantsResponse struct {
	GeneralCount int                 `json:"general_count,omitempty"`
	Fixed        []string            `json:"fixed,omitempty"`
	Choices      *featChoicesResponse `json:"choices,omitempty"`
}

type featChoicesResponse struct {
	Pool  []string `json:"pool"`
	Count int      `json:"count"`
}

type techGrantsResponse struct {
	Hardwired   []string               `json:"hardwired,omitempty"`
	Prepared    *preparedGrantsResponse    `json:"prepared,omitempty"`
	Spontaneous *spontaneousGrantsResponse `json:"spontaneous,omitempty"`
}

type preparedGrantsResponse struct {
	SlotsByLevel map[int]int             `json:"slots_by_level,omitempty"`
	Fixed        []preparedEntryResponse `json:"fixed,omitempty"`
	Pool         []preparedEntryResponse `json:"pool,omitempty"`
}

type preparedEntryResponse struct {
	ID    string `json:"id"`
	Level int    `json:"level"`
}

type spontaneousGrantsResponse struct {
	KnownByLevel map[int]int                `json:"known_by_level,omitempty"`
	UsesByLevel  map[int]int                `json:"uses_by_level,omitempty"`
	Fixed        []spontaneousEntryResponse `json:"fixed,omitempty"`
	Pool         []spontaneousEntryResponse `json:"pool,omitempty"`
}

type spontaneousEntryResponse struct {
	ID    string `json:"id"`
	Level int    `json:"level"`
}

type regionResponse struct {
	ID            string                     `json:"id"`
	Name          string                     `json:"name"`
	Description   string                     `json:"description"`
	Modifiers     map[string]int             `json:"modifiers,omitempty"`
	AbilityBoosts *abilityBoostGrantResponse `json:"ability_boosts,omitempty"`
}

type jobResponse struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	Description       string               `json:"description"`
	Archetype         string               `json:"archetype"`
	Team              string               `json:"team"`
	KeyAbility        string               `json:"key_ability"`
	HitPointsPerLevel int                  `json:"hit_points_per_level"`
	SkillGrants       *skillGrantsResponse `json:"skill_grants,omitempty"`
	FeatGrants        *featGrantsResponse  `json:"feat_grants,omitempty"`
	TechGrants        *techGrantsResponse  `json:"tech_grants,omitempty"`
}

type archetypeResponse struct {
	ID            string                     `json:"id"`
	Name          string                     `json:"name"`
	Description   string                     `json:"description"`
	AbilityBoosts *abilityBoostGrantResponse `json:"ability_boosts,omitempty"`
}

type teamResponse struct {
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
		var abr *abilityBoostGrantResponse
		if reg.AbilityBoosts != nil {
			abr = &abilityBoostGrantResponse{
				Fixed: reg.AbilityBoosts.Fixed,
				Free:  reg.AbilityBoosts.Free,
			}
		}
		regions = append(regions, regionResponse{
			ID:            reg.ID,
			Name:          reg.Name,
			Description:   reg.Description,
			Modifiers:     reg.Modifiers,
			AbilityBoosts: abr,
		})
	}
	jobs := make([]jobResponse, 0, len(h.options.Jobs))
	for _, job := range h.options.Jobs {
		var sg *skillGrantsResponse
		if job.SkillGrants != nil {
			sg = &skillGrantsResponse{
				Fixed: job.SkillGrants.Fixed,
			}
			if job.SkillGrants.Choices != nil {
				sg.Choices = &skillChoicesResponse{
					Pool:  job.SkillGrants.Choices.Pool,
					Count: job.SkillGrants.Choices.Count,
				}
			}
		}
		var fg *featGrantsResponse
		if job.FeatGrants != nil {
			fg = &featGrantsResponse{
				GeneralCount: job.FeatGrants.GeneralCount,
				Fixed:        job.FeatGrants.Fixed,
			}
			if job.FeatGrants.Choices != nil {
				fg.Choices = &featChoicesResponse{
					Pool:  job.FeatGrants.Choices.Pool,
					Count: job.FeatGrants.Choices.Count,
				}
			}
		}
		var tg *techGrantsResponse
		if job.TechnologyGrants != nil {
			tg = &techGrantsResponse{
				Hardwired: job.TechnologyGrants.Hardwired,
			}
			if job.TechnologyGrants.Prepared != nil {
				prep := &preparedGrantsResponse{
					SlotsByLevel: job.TechnologyGrants.Prepared.SlotsByLevel,
				}
				for _, e := range job.TechnologyGrants.Prepared.Fixed {
					prep.Fixed = append(prep.Fixed, preparedEntryResponse{ID: e.ID, Level: e.Level})
				}
				for _, e := range job.TechnologyGrants.Prepared.Pool {
					prep.Pool = append(prep.Pool, preparedEntryResponse{ID: e.ID, Level: e.Level})
				}
				tg.Prepared = prep
			}
			if job.TechnologyGrants.Spontaneous != nil {
				spont := &spontaneousGrantsResponse{
					KnownByLevel: job.TechnologyGrants.Spontaneous.KnownByLevel,
					UsesByLevel:  job.TechnologyGrants.Spontaneous.UsesByLevel,
				}
				for _, e := range job.TechnologyGrants.Spontaneous.Fixed {
					spont.Fixed = append(spont.Fixed, spontaneousEntryResponse{ID: e.ID, Level: e.Level})
				}
				for _, e := range job.TechnologyGrants.Spontaneous.Pool {
					spont.Pool = append(spont.Pool, spontaneousEntryResponse{ID: e.ID, Level: e.Level})
				}
				tg.Spontaneous = spont
			}
		}
		jobs = append(jobs, jobResponse{
			ID:                job.ID,
			Name:              job.Name,
			Description:       job.Description,
			Archetype:         job.Archetype,
			Team:              job.Team,
			KeyAbility:        job.KeyAbility,
			HitPointsPerLevel: job.HitPointsPerLevel,
			SkillGrants:       sg,
			FeatGrants:        fg,
			TechGrants:        tg,
		})
	}
	archetypes := make([]archetypeResponse, 0, len(h.options.Archetypes))
	for _, arch := range h.options.Archetypes {
		var aba *abilityBoostGrantResponse
		if arch.AbilityBoosts != nil {
			aba = &abilityBoostGrantResponse{
				Fixed: arch.AbilityBoosts.Fixed,
				Free:  arch.AbilityBoosts.Free,
			}
		}
		archetypes = append(archetypes, archetypeResponse{
			ID:            arch.ID,
			Name:          arch.Name,
			Description:   arch.Description,
			AbilityBoosts: aba,
		})
	}
	teams := make([]teamResponse, 0, len(h.options.Teams))
	for _, t := range h.options.Teams {
		teams = append(teams, teamResponse{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"regions":    regions,
		"jobs":       jobs,
		"archetypes": archetypes,
		"teams":      teams,
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
