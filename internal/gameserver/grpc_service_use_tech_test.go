package gameserver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// fakePrepRepoUse is a PreparedTechRepo fake for use-tech tests.
type fakePrepRepoUse struct {
	slots            map[int][]*session.PreparedSlot
	setExpendedCalls int
}

func (r *fakePrepRepoUse) GetAll(_ context.Context, _ int64) (map[int][]*session.PreparedSlot, error) {
	return r.slots, nil
}
func (r *fakePrepRepoUse) Set(_ context.Context, _ int64, level, index int, techID string) error {
	if r.slots == nil {
		r.slots = make(map[int][]*session.PreparedSlot)
	}
	for len(r.slots[level]) <= index {
		r.slots[level] = append(r.slots[level], nil)
	}
	r.slots[level][index] = &session.PreparedSlot{TechID: techID}
	return nil
}
func (r *fakePrepRepoUse) DeleteAll(_ context.Context, _ int64) error {
	r.slots = nil
	return nil
}
func (r *fakePrepRepoUse) SetExpended(_ context.Context, _ int64, level, index int, expended bool) error {
	if r.slots != nil {
		if slots, ok := r.slots[level]; ok && index < len(slots) && slots[index] != nil {
			slots[index].Expended = expended
		}
	}
	r.setExpendedCalls++
	return nil
}

// setupUseTechPlayer creates a service and player for use-tech tests.
func setupUseTechPlayer(t *testing.T, prepRepo *fakePrepRepoUse) (*GameServiceServer, *session.Manager, string) {
	t.Helper()
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	svc.SetPreparedTechRepo(prepRepo)
	uid := "player-use-tech"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.PreparedTechs = prepRepo.slots
	return svc, sessMgr, uid
}

// REQ-UC1: use <tech> with a non-expended prepared slot expends it and returns activation message.
func TestHandleUse_PreparedTech_ExpendsSlot(t *testing.T) {
	prepRepo := &fakePrepRepoUse{
		slots: map[int][]*session.PreparedSlot{
			1: {{TechID: "shock_grenade", Expended: false}},
		},
	}
	svc, sessMgr, uid := setupUseTechPlayer(t, prepRepo)

	evt, err := svc.handleUse(uid, "shock_grenade", "")
	require.NoError(t, err)
	require.NotNil(t, evt)

	sess, _ := sessMgr.GetPlayer(uid)
	require.NotNil(t, sess.PreparedTechs[1][0])
	assert.True(t, sess.PreparedTechs[1][0].Expended, "slot must be marked expended")
	assert.Equal(t, 1, prepRepo.setExpendedCalls)

	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "shock_grenade")
}

// REQ-UC2: use <tech> with all slots expended returns "No prepared uses remaining".
func TestHandleUse_PreparedTech_AllExpended_ReturnsNoRemaining(t *testing.T) {
	prepRepo := &fakePrepRepoUse{
		slots: map[int][]*session.PreparedSlot{
			1: {{TechID: "shock_grenade", Expended: true}},
		},
	}
	svc, _, uid := setupUseTechPlayer(t, prepRepo)

	evt, err := svc.handleUse(uid, "shock_grenade", "")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "No prepared uses")
	assert.Equal(t, 0, prepRepo.setExpendedCalls)
}

// REQ-UC3: use <tech> with no slot for that tech returns "No prepared uses remaining".
func TestHandleUse_PreparedTech_NoSlotForTech_ReturnsNoRemaining(t *testing.T) {
	prepRepo := &fakePrepRepoUse{
		slots: map[int][]*session.PreparedSlot{
			1: {{TechID: "other_tech", Expended: false}},
		},
	}
	svc, _, uid := setupUseTechPlayer(t, prepRepo)

	evt, err := svc.handleUse(uid, "shock_grenade", "")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "No prepared uses")
}

// REQ-UC4: use (no arg) includes prepared techs with remaining use counts in choices list.
func TestHandleUse_NoArg_IncludesPreparedTechs(t *testing.T) {
	prepRepo := &fakePrepRepoUse{
		slots: map[int][]*session.PreparedSlot{
			1: {
				{TechID: "shock_grenade", Expended: false},
				{TechID: "shock_grenade", Expended: true},
				{TechID: "neural_disruptor", Expended: false},
			},
		},
	}
	svc, _, uid := setupUseTechPlayer(t, prepRepo)

	evt, err := svc.handleUse(uid, "", "")
	require.NoError(t, err)
	require.NotNil(t, evt)

	resp := evt.GetUseResponse()
	require.NotNil(t, resp)

	var foundShock, foundNeural bool
	for _, c := range resp.Choices {
		if c.FeatId == "shock_grenade" {
			foundShock = true
			assert.Contains(t, c.Description, "1", "should indicate 1 use remaining")
		}
		if c.FeatId == "neural_disruptor" {
			foundNeural = true
		}
	}
	assert.True(t, foundShock, "shock_grenade must appear in choices")
	assert.True(t, foundNeural, "neural_disruptor must appear in choices")
}

// REQ-UC7 (property): For N non-expended slots, N uses expend all; (N+1)th call returns "no remaining".
func TestPropertyHandleUse_PreparedTech_ExpendsExactly(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 4).Draw(rt, "n")
		slots := make([]*session.PreparedSlot, n)
		for i := range slots {
			slots[i] = &session.PreparedSlot{TechID: "test_tech", Expended: false}
		}
		prepRepo := &fakePrepRepoUse{
			slots: map[int][]*session.PreparedSlot{1: slots},
		}
		sessMgr := session.NewManager()
		svc := testMinimalService(t, sessMgr)
		svc.SetPreparedTechRepo(prepRepo)
		uid := fmt.Sprintf("prop-use-%d", rapid.IntRange(0, 9999).Draw(rt, "uid"))
		_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
		})
		if err != nil {
			rt.Skip()
		}
		sess, ok := sessMgr.GetPlayer(uid)
		if !ok {
			rt.Skip()
		}
		sess.PreparedTechs = prepRepo.slots

		// Call use n times — all should succeed.
		for i := 0; i < n; i++ {
			evt, err := svc.handleUse(uid, "test_tech", "")
			if err != nil {
				rt.Fatalf("call %d: unexpected error: %v", i, err)
			}
			if msg := evt.GetMessage(); msg != nil && strings.Contains(msg.Content, "No prepared uses") {
				rt.Fatalf("call %d of %d: got 'no remaining' too early", i, n)
			}
		}

		// (N+1)th call must return "no remaining".
		evt, err := svc.handleUse(uid, "test_tech", "")
		if err != nil {
			rt.Fatalf("(n+1)th call: unexpected error: %v", err)
		}
		msg := evt.GetMessage()
		if msg == nil || !strings.Contains(msg.Content, "No prepared uses") {
			rt.Fatalf("(n+1)th call: expected 'No prepared uses', got: %v", msg)
		}

		// Exactly n slots must be expended.
		expiredCount := 0
		for _, slot := range sess.PreparedTechs[1] {
			if slot != nil && slot.Expended {
				expiredCount++
			}
		}
		if expiredCount != n {
			rt.Fatalf("expected %d expended slots, got %d", n, expiredCount)
		}
	})
}
