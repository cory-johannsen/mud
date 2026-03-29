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
