package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/cmd/webclient/handlers"
	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	postgres "github.com/cory-johannsen/mud/internal/storage/postgres"
)

// stubCharacterRepo implements handlers.CharacterLister for tests.
type stubCharacterRepo struct {
	chars []*character.Character
	err   error
}

func (s *stubCharacterRepo) ListByAccount(ctx context.Context, accountID int64) ([]*character.Character, error) {
	return s.chars, s.err
}

func TestListCharacters_ReturnsOwnedCharacters(t *testing.T) {
	repo := &stubCharacterRepo{
		chars: []*character.Character{
			{
				ID: 1, Name: "Zork", Class: "ganger", Level: 5,
				CurrentHP: 38, MaxHP: 50, Region: "rustbucket", Team: "gun",
			},
		},
	}
	h := handlers.NewCharacterHandler(repo, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/characters", nil)
	req = req.WithContext(handlers.WithAccountID(req.Context(), 42))
	rr := httptest.NewRecorder()

	h.ListCharacters(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp []handlers.CharacterResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, int64(1), resp[0].ID)
	assert.Equal(t, "Zork", resp[0].Name)
	assert.Equal(t, "ganger", resp[0].Job)
	assert.Equal(t, 5, resp[0].Level)
	assert.Equal(t, int32(38), resp[0].CurrentHP)
	assert.Equal(t, int32(50), resp[0].MaxHP)
	assert.Equal(t, "rustbucket", resp[0].Region)
	assert.Equal(t, "gun", resp[0].Archetype)
}

func TestListCharacters_EmptyList(t *testing.T) {
	repo := &stubCharacterRepo{chars: []*character.Character{}}
	h := handlers.NewCharacterHandler(repo, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/characters", nil)
	req = req.WithContext(handlers.WithAccountID(req.Context(), 99))
	rr := httptest.NewRecorder()

	h.ListCharacters(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp []handlers.CharacterResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Len(t, resp, 0)
}

type stubCreator struct {
	result *character.Character
	err    error
}

func (s *stubCreator) Create(ctx context.Context, c *character.Character) (*character.Character, error) {
	return s.result, s.err
}

func TestCreateCharacter_Success(t *testing.T) {
	created := &character.Character{
		ID: 7, Name: "Mira", Class: "ganger", Team: "gun",
		Region: "rustbucket", Gender: "female", Level: 1,
		CurrentHP: 20, MaxHP: 20,
	}
	creator := &stubCreator{result: created}
	lister := &stubCharacterRepo{}
	h := handlers.NewCharacterHandler(lister, creator, nil)

	body := `{"name":"Mira","job":"ganger","team":"gun","region":"rustbucket","gender":"female"}`
	req := httptest.NewRequest(http.MethodPost, "/api/characters", strings.NewReader(body))
	req = req.WithContext(handlers.WithAccountID(req.Context(), 42))
	rr := httptest.NewRecorder()

	h.CreateCharacter(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	var resp struct {
		Character handlers.CharacterResponse `json:"character"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, int64(7), resp.Character.ID)
	assert.Equal(t, "Mira", resp.Character.Name)
}

func TestCreateCharacter_NameTooShort(t *testing.T) {
	h := handlers.NewCharacterHandler(&stubCharacterRepo{}, &stubCreator{}, nil)
	body := `{"name":"ab","job":"ganger","team":"gun","region":"rustbucket","gender":"female"}`
	req := httptest.NewRequest(http.MethodPost, "/api/characters", strings.NewReader(body))
	req = req.WithContext(handlers.WithAccountID(req.Context(), 42))
	rr := httptest.NewRecorder()
	h.CreateCharacter(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateCharacter_NameTooLong(t *testing.T) {
	h := handlers.NewCharacterHandler(&stubCharacterRepo{}, &stubCreator{}, nil)
	body := `{"name":"ThisNameIsWayTooLongForValidation","job":"ganger","team":"gun","region":"rustbucket","gender":"female"}`
	req := httptest.NewRequest(http.MethodPost, "/api/characters", strings.NewReader(body))
	req = req.WithContext(handlers.WithAccountID(req.Context(), 42))
	rr := httptest.NewRecorder()
	h.CreateCharacter(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateCharacter_MissingRequiredField(t *testing.T) {
	h := handlers.NewCharacterHandler(&stubCharacterRepo{}, &stubCreator{}, nil)
	body := `{"name":"Mira","job":"ganger","team":"gun","region":"rustbucket"}`
	req := httptest.NewRequest(http.MethodPost, "/api/characters", strings.NewReader(body))
	req = req.WithContext(handlers.WithAccountID(req.Context(), 42))
	rr := httptest.NewRecorder()
	h.CreateCharacter(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateCharacter_NameTaken(t *testing.T) {
	creator := &stubCreator{err: postgres.ErrCharacterNameTaken}
	h := handlers.NewCharacterHandler(&stubCharacterRepo{}, creator, nil)
	body := `{"name":"Zork","job":"ganger","team":"gun","region":"rustbucket","gender":"male"}`
	req := httptest.NewRequest(http.MethodPost, "/api/characters", strings.NewReader(body))
	req = req.WithContext(handlers.WithAccountID(req.Context(), 42))
	rr := httptest.NewRecorder()
	h.CreateCharacter(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

// stubBoostsAdder records calls to Add.
type stubBoostsAdder struct {
	calls []struct{ source, ability string }
}

func (s *stubBoostsAdder) Add(_ context.Context, _ int64, source, ability string) error {
	s.calls = append(s.calls, struct{ source, ability string }{source, ability})
	return nil
}

// stubSkillsSetter records the last SetAll call.
type stubSkillsSetter struct {
	calledWith map[string]string
}

func (s *stubSkillsSetter) SetAll(_ context.Context, _ int64, skills map[string]string) error {
	s.calledWith = skills
	return nil
}

// stubFeatsSetter records the last SetAll call.
type stubFeatsSetter struct {
	calledWith []string
}

func (s *stubFeatsSetter) SetAll(_ context.Context, _ int64, feats []string) error {
	s.calledWith = feats
	return nil
}

// stubHWTechSetter records the last SetAll call.
type stubHWTechSetter struct {
	calledWith []string
}

func (s *stubHWTechSetter) SetAll(_ context.Context, _ int64, techIDs []string) error {
	s.calledWith = techIDs
	return nil
}

// stubSpontAdder records calls to Add.
type stubSpontAdder struct {
	calls []struct {
		id    string
		level int
	}
}

func (s *stubSpontAdder) Add(_ context.Context, _ int64, techID string, level int) error {
	s.calls = append(s.calls, struct {
		id    string
		level int
	}{techID, level})
	return nil
}

func TestCreateCharacter_PersistsChoicesWhenReposSet(t *testing.T) {
	created := &character.Character{
		ID: 42, Name: "Kira", Class: "ganger", Team: "gun",
		Region: "rustbucket", Gender: "female", Level: 1,
		CurrentHP: 20, MaxHP: 20,
	}
	creator := &stubCreator{result: created}
	lister := &stubCharacterRepo{}

	boostsAdder := &stubBoostsAdder{}
	skillsSetter := &stubSkillsSetter{}
	featsSetter := &stubFeatsSetter{}
	hwTechSetter := &stubHWTechSetter{}
	spontAdder := &stubSpontAdder{}

	opts := &handlers.CharacterOptions{
		Jobs: []*ruleset.Job{
			{
				ID: "ganger",
				SkillGrants: &ruleset.SkillGrants{
					Fixed: []string{"intimidation"},
				},
				FeatGrants: &ruleset.FeatGrants{
					Fixed: []string{"feat_brawler"},
				},
				TechnologyGrants: &ruleset.TechnologyGrants{
					Hardwired: []string{"tech_ganger_neural"},
				},
			},
		},
		Skills: []*ruleset.Skill{
			{ID: "intimidation"},
			{ID: "athletics"},
			{ID: "stealth"},
		},
	}

	h := handlers.NewCharacterHandler(lister, creator, nil).
		WithOptions(opts).
		WithPersistenceRepos(boostsAdder, skillsSetter, featsSetter, hwTechSetter, spontAdder)

	body := `{
		"name":"Kira","job":"ganger","team":"gun","region":"rustbucket","gender":"female",
		"archetype_boosts":["brutality"],
		"region_boosts":["grit"],
		"skill_choices":["athletics"],
		"feat_choices":["feat_street_savvy"],
		"general_feat_choices":["feat_toughness"],
		"spontaneous_choices":[{"id":"tech_stim","level":1}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/characters", strings.NewReader(body))
	req = req.WithContext(handlers.WithAccountID(req.Context(), 10))
	rr := httptest.NewRecorder()

	h.CreateCharacter(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)

	// Ability boosts: one archetype + one region boost.
	require.Len(t, boostsAdder.calls, 2)
	assert.Equal(t, "archetype", boostsAdder.calls[0].source)
	assert.Equal(t, "brutality", boostsAdder.calls[0].ability)
	assert.Equal(t, "region", boostsAdder.calls[1].source)
	assert.Equal(t, "grit", boostsAdder.calls[1].ability)

	// Skills: all three skills set; intimidation + athletics = trained, stealth = untrained.
	require.NotNil(t, skillsSetter.calledWith)
	assert.Equal(t, "trained", skillsSetter.calledWith["intimidation"])
	assert.Equal(t, "trained", skillsSetter.calledWith["athletics"])
	assert.Equal(t, "untrained", skillsSetter.calledWith["stealth"])

	// Feats: fixed + player choice + general choice.
	require.NotNil(t, featsSetter.calledWith)
	assert.Contains(t, featsSetter.calledWith, "feat_brawler")
	assert.Contains(t, featsSetter.calledWith, "feat_street_savvy")
	assert.Contains(t, featsSetter.calledWith, "feat_toughness")

	// Hardwired tech: from job grants.
	require.NotNil(t, hwTechSetter.calledWith)
	assert.Equal(t, []string{"tech_ganger_neural"}, hwTechSetter.calledWith)

	// Spontaneous tech: player choice.
	require.Len(t, spontAdder.calls, 1)
	assert.Equal(t, "tech_stim", spontAdder.calls[0].id)
	assert.Equal(t, 1, spontAdder.calls[0].level)
}

func TestCreateCharacter_NoPersistenceWhenReposNil(t *testing.T) {
	created := &character.Character{
		ID: 5, Name: "Rex", Class: "ganger", Team: "gun",
		Region: "rustbucket", Gender: "male", Level: 1,
	}
	creator := &stubCreator{result: created}
	// No WithPersistenceRepos call — repos are all nil.
	h := handlers.NewCharacterHandler(&stubCharacterRepo{}, creator, nil)

	body := `{
		"name":"Rex","job":"ganger","team":"gun","region":"rustbucket","gender":"male",
		"archetype_boosts":["brutality"],
		"skill_choices":["athletics"],
		"spontaneous_choices":[{"id":"tech_stim","level":1}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/characters", strings.NewReader(body))
	req = req.WithContext(handlers.WithAccountID(req.Context(), 10))
	rr := httptest.NewRecorder()

	// Must not panic even with no repos attached.
	h.CreateCharacter(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)
}

type stubNameChecker struct {
	available bool
	err       error
}

func (s *stubNameChecker) IsNameAvailable(ctx context.Context, name string) (bool, error) {
	return s.available, s.err
}

func TestListOptions_ReturnsRulesetData(t *testing.T) {
	opts := &handlers.CharacterOptions{
		Regions:    []*ruleset.Region{{ID: "rustbucket", Name: "Rustbucket Ridge"}},
		Jobs:       []*ruleset.Job{{ID: "ganger", Name: "Ganger"}},
		Archetypes: []*ruleset.Archetype{{ID: "aggressor", Name: "Aggressor"}},
		Teams:      []*ruleset.Team{{ID: "gun", Name: "Gun"}},
	}
	h := handlers.NewCharacterHandler(nil, nil, nil).WithOptions(opts)

	req := httptest.NewRequest(http.MethodGet, "/api/characters/options", nil)
	rr := httptest.NewRecorder()
	h.ListOptions(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Contains(t, body, "regions")
	assert.Contains(t, body, "jobs")
	assert.Contains(t, body, "archetypes")
	assert.Contains(t, body, "teams")
}

func TestListOptions_SkillsArrayIncludesIDNameDescription(t *testing.T) {
	opts := &handlers.CharacterOptions{
		Regions:    []*ruleset.Region{{ID: "rustbucket", Name: "Rustbucket Ridge"}},
		Jobs:       []*ruleset.Job{{ID: "ganger", Name: "Ganger"}},
		Archetypes: []*ruleset.Archetype{{ID: "aggressor", Name: "Aggressor"}},
		Teams:      []*ruleset.Team{{ID: "gun", Name: "Gun"}},
		Skills: []*ruleset.Skill{
			{ID: "athletics", Name: "Athletics", Description: "Physical prowess and raw strength.", Ability: "str"},
			{ID: "intimidation", Name: "Intimidation", Description: "Coerce others through threats.", Ability: "cha"},
		},
	}
	h := handlers.NewCharacterHandler(nil, nil, nil).WithOptions(opts)

	req := httptest.NewRequest(http.MethodGet, "/api/characters/options", nil)
	rr := httptest.NewRecorder()
	h.ListOptions(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	require.Contains(t, body, "skills")

	rawSkills, ok := body["skills"].([]any)
	require.True(t, ok, "skills must be a JSON array")
	require.Len(t, rawSkills, 2)

	first, ok := rawSkills[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "athletics", first["id"])
	assert.Equal(t, "Athletics", first["name"])
	assert.Equal(t, "Physical prowess and raw strength.", first["description"])
	assert.Equal(t, "str", first["ability"])

	second, ok := rawSkills[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "intimidation", second["id"])
	assert.Equal(t, "Intimidation", second["name"])
}

func TestListOptions_SkillsArrayEmptyWhenNilSkills(t *testing.T) {
	opts := &handlers.CharacterOptions{
		Regions:    []*ruleset.Region{{ID: "rustbucket", Name: "Rustbucket Ridge"}},
		Jobs:       []*ruleset.Job{{ID: "ganger", Name: "Ganger"}},
		Archetypes: []*ruleset.Archetype{{ID: "aggressor", Name: "Aggressor"}},
		Teams:      []*ruleset.Team{{ID: "gun", Name: "Gun"}},
		Skills:     nil,
	}
	h := handlers.NewCharacterHandler(nil, nil, nil).WithOptions(opts)

	req := httptest.NewRequest(http.MethodGet, "/api/characters/options", nil)
	rr := httptest.NewRecorder()
	h.ListOptions(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	require.Contains(t, body, "skills")
	rawSkills, ok := body["skills"].([]any)
	require.True(t, ok, "skills must be a JSON array even when no skills loaded")
	assert.Len(t, rawSkills, 0)
}

func TestCheckName_Available(t *testing.T) {
	checker := &stubNameChecker{available: true}
	h := handlers.NewCharacterHandler(nil, nil, checker)

	req := httptest.NewRequest(http.MethodGet, "/api/characters/check-name?name=Zork", nil)
	rr := httptest.NewRecorder()
	h.CheckName(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var body map[string]bool
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.True(t, body["available"])
}

func TestCheckName_Taken(t *testing.T) {
	checker := &stubNameChecker{available: false}
	h := handlers.NewCharacterHandler(nil, nil, checker)

	req := httptest.NewRequest(http.MethodGet, "/api/characters/check-name?name=Zork", nil)
	rr := httptest.NewRecorder()
	h.CheckName(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var body map[string]bool
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.False(t, body["available"])
}

func TestCheckName_MissingParam(t *testing.T) {
	h := handlers.NewCharacterHandler(nil, nil, &stubNameChecker{})
	req := httptest.NewRequest(http.MethodGet, "/api/characters/check-name", nil)
	rr := httptest.NewRecorder()
	h.CheckName(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// stubDeleter implements handlers.CharacterDeleter for tests.
type stubDeleter struct {
	err      error
	called   bool
	lastAcct int64
	lastID   int64
}

func (s *stubDeleter) DeleteByID(_ context.Context, accountID, charID int64) error {
	s.called = true
	s.lastAcct = accountID
	s.lastID = charID
	return s.err
}

func TestDeleteCharacter_Success(t *testing.T) {
	d := &stubDeleter{}
	h := handlers.NewCharacterHandler(nil, nil, nil).WithDeleter(d)
	req := httptest.NewRequest(http.MethodDelete, "/api/characters/5", nil)
	req.SetPathValue("id", "5")
	req = req.WithContext(handlers.WithAccountID(req.Context(), 42))
	rr := httptest.NewRecorder()
	h.DeleteCharacter(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.True(t, d.called)
	assert.Equal(t, int64(42), d.lastAcct)
	assert.Equal(t, int64(5), d.lastID)
}

func TestDeleteCharacter_NotFound(t *testing.T) {
	d := &stubDeleter{err: postgres.ErrCharacterNotFound}
	h := handlers.NewCharacterHandler(nil, nil, nil).WithDeleter(d)
	req := httptest.NewRequest(http.MethodDelete, "/api/characters/5", nil)
	req.SetPathValue("id", "5")
	req = req.WithContext(handlers.WithAccountID(req.Context(), 42))
	rr := httptest.NewRecorder()
	h.DeleteCharacter(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDeleteCharacter_NotConfigured(t *testing.T) {
	h := handlers.NewCharacterHandler(nil, nil, nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/characters/5", nil)
	req.SetPathValue("id", "5")
	rr := httptest.NewRecorder()
	h.DeleteCharacter(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
