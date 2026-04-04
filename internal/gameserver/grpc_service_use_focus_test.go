package gameserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

// focusFakeCharSaver is a CharacterSaver stub that records SaveFocusPoints calls.
type focusFakeCharSaver struct {
	fakeCharSaver
	saveFocusPointsCalls []saveFocusPointsCall
}

type saveFocusPointsCall struct {
	characterID int64
	fp          int
}

func (f *focusFakeCharSaver) SaveFocusPoints(_ context.Context, characterID int64, fp int) error {
	f.saveFocusPointsCalls = append(f.saveFocusPointsCalls, saveFocusPointsCall{characterID, fp})
	return nil
}
func (f *focusFakeCharSaver) SaveHotbar(_ context.Context, _ int64, _ [10]session.HotbarSlot) error {
	return nil
}
func (f *focusFakeCharSaver) LoadHotbar(_ context.Context, _ int64) ([10]session.HotbarSlot, error) {
	return [10]session.HotbarSlot{}, nil
}

// fakeInnateRepoFocus is a minimal innate-tech repo stub for FP tests.
type fakeInnateRepoFocus struct {
	slots          map[string]*session.InnateSlot
	decrementCalls int
}

func (r *fakeInnateRepoFocus) GetAll(_ context.Context, _ int64) (map[string]*session.InnateSlot, error) {
	return r.slots, nil
}
func (r *fakeInnateRepoFocus) Set(_ context.Context, _ int64, techID string, maxUses int) error {
	if r.slots == nil {
		r.slots = make(map[string]*session.InnateSlot)
	}
	r.slots[techID] = &session.InnateSlot{MaxUses: maxUses, UsesRemaining: maxUses}
	return nil
}
func (r *fakeInnateRepoFocus) DeleteAll(_ context.Context, _ int64) error { r.slots = nil; return nil }
func (r *fakeInnateRepoFocus) Decrement(_ context.Context, _ int64, _ string) error {
	r.decrementCalls++
	return nil
}
func (r *fakeInnateRepoFocus) RestoreAll(_ context.Context, _ int64) error { return nil }

// setupFocusTechPlayer creates a GameServiceServer with a tech registry, innate-tech repo,
// and charSaver, and registers the player session with the given FP values.
func setupFocusTechPlayer(
	t *testing.T,
	techDef *technology.TechnologyDef,
	innateFP, maxFP int,
) (*GameServiceServer, *focusFakeCharSaver, string) {
	t.Helper()

	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	reg := technology.NewRegistry()
	reg.Register(techDef)
	svc.SetTechRegistry(reg)

	innRepo := &fakeInnateRepoFocus{
		slots: map[string]*session.InnateSlot{
			techDef.ID: {MaxUses: 0, UsesRemaining: 0}, // unlimited uses
		},
	}
	svc.SetInnateTechRepo(innRepo)

	charSaver := &focusFakeCharSaver{}
	svc.SetCharSaver(charSaver)

	const uid = "player-focus-tech"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    uid,
		RoomID:      "room_a",
		CharacterID: 42,
		Role:        "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.FocusPoints = innateFP
	sess.MaxFocusPoints = maxFP
	sess.InnateTechs = innRepo.slots

	return svc, charSaver, uid
}

// REQ-FP-4: Activating a focus-cost technology with 0 FP returns an error message.
func TestHandleUse_FocusTech_NoFP_Fails(t *testing.T) {
	techDef := &technology.TechnologyDef{
		ID:         "ki_strike",
		Name:       "Ki Strike",
		FocusCost:  true,
		ActionCost: 2,
	}
	svc, charSaver, uid := setupFocusTechPlayer(t, techDef, 0, 1)

	evt, err := svc.handleUse(uid, "ki_strike", "")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected a message event")
	assert.Contains(t, msg.Content, "Not enough Focus Points. (0/1)")
	assert.Empty(t, charSaver.saveFocusPointsCalls, "SaveFocusPoints must not be called on failure")
}

// REQ-FP-3/REQ-FP-5/REQ-FP-6: Activating a focus-cost technology with FP > 0 decrements FP and persists.
func TestHandleUse_FocusTech_WithFP_Succeeds(t *testing.T) {
	techDef := &technology.TechnologyDef{
		ID:         "ki_strike",
		Name:       "Ki Strike",
		FocusCost:  true,
		ActionCost: 2,
	}
	svc, charSaver, uid := setupFocusTechPlayer(t, techDef, 1, 1)

	evt, err := svc.handleUse(uid, "ki_strike", "")
	require.NoError(t, err)
	require.NotNil(t, evt)

	// FP must be decremented in-session.
	sessMgr := svc.sessions
	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, 0, sess.FocusPoints, "FocusPoints must be decremented to 0")

	// SaveFocusPoints must be called exactly once with the new value.
	require.Len(t, charSaver.saveFocusPointsCalls, 1, "SaveFocusPoints must be called exactly once")
	assert.Equal(t, int64(42), charSaver.saveFocusPointsCalls[0].characterID)
	assert.Equal(t, 0, charSaver.saveFocusPointsCalls[0].fp)
}
