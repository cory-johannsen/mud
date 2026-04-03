package gameserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// stubSpontaneousUsePoolRepo is a fully in-memory SpontaneousUsePoolRepo for BUG-107 tests.
type stubSpontaneousUsePoolRepo struct {
	pools map[int]session.UsePool
}

func newStubSpontaneousUsePoolRepo(pools map[int]session.UsePool) *stubSpontaneousUsePoolRepo {
	return &stubSpontaneousUsePoolRepo{pools: pools}
}

func (r *stubSpontaneousUsePoolRepo) GetAll(_ context.Context, _ int64) (map[int]session.UsePool, error) {
	out := make(map[int]session.UsePool, len(r.pools))
	for k, v := range r.pools {
		out[k] = v
	}
	return out, nil
}

func (r *stubSpontaneousUsePoolRepo) Set(_ context.Context, _ int64, techLevel, usesRemaining, maxUses int) error {
	r.pools[techLevel] = session.UsePool{Remaining: usesRemaining, Max: maxUses}
	return nil
}

func (r *stubSpontaneousUsePoolRepo) Decrement(_ context.Context, _ int64, techLevel int) error {
	if p, ok := r.pools[techLevel]; ok && p.Remaining > 0 {
		p.Remaining--
		r.pools[techLevel] = p
	}
	return nil
}

func (r *stubSpontaneousUsePoolRepo) RestoreAll(_ context.Context, _ int64) error {
	for level, p := range r.pools {
		p.Remaining = p.Max
		r.pools[level] = p
	}
	return nil
}

func (r *stubSpontaneousUsePoolRepo) RestorePartial(_ context.Context, _ int64, _ float64) error {
	return nil
}

func (r *stubSpontaneousUsePoolRepo) DeleteAll(_ context.Context, _ int64) error {
	r.pools = make(map[int]session.UsePool)
	return nil
}

// newBug107World builds a single-zone world with one room for respawn tests.
func newBug107World() (*world.Manager, string) {
	room := &world.Room{ID: "spawn-room", ZoneID: "zone-test", Title: "Start", Description: "D"}
	zone := &world.Zone{
		ID:        "zone-test",
		Name:      "Test Zone",
		StartRoom: "spawn-room",
		Rooms:     map[string]*world.Room{"spawn-room": room},
	}
	mgr, err := world.NewManager([]*world.Zone{zone})
	if err != nil {
		panic(err)
	}
	return mgr, "spawn-room"
}

// TestRespawnPlayer_RestoresFullHP verifies that respawnPlayer sets CurrentHP to MaxHP (BUG-107).
//
// Precondition: sess.Dead=true, sess.CurrentHP=0, sess.MaxHP=42.
// Postcondition: sess.CurrentHP==42, sess.Dead==false.
func TestRespawnPlayer_RestoresFullHP(t *testing.T) {
	wMgr, spawnRoomID := newBug107World()
	sMgr := session.NewManager()
	npcMgr := npc.NewManager()
	worldH := NewWorldHandler(wMgr, sMgr, npcMgr, nil, nil, &inventory.Registry{})
	logger := zap.NewNop()

	sess, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "uid-bug107",
		Username:    "tester",
		CharName:    "Hero",
		CharacterID: 0, // 0 skips DB persistence checks
		RoomID:      spawnRoomID,
		CurrentHP:   0,
		MaxHP:       42,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)

	sess.Dead = true
	sess.Conditions = condition.NewActiveSet()
	// Apply a permanent condition that should be cleared on respawn.
	permDef := &condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3}
	require.NoError(t, sess.Conditions.Apply("uid-bug107", permDef, 2, -1))
	require.True(t, sess.Conditions.Has("wounded"), "pre-condition: wounded must be active before respawn")

	svc := &GameServiceServer{
		world:    wMgr,
		sessions: sMgr,
		worldH:   worldH,
		logger:   logger,
	}

	svc.respawnPlayer("uid-bug107")

	assert.False(t, sess.Dead, "sess.Dead must be false after respawn")
	assert.Equal(t, 42, sess.CurrentHP, "BUG-107: CurrentHP must equal MaxHP after respawn, not 1")
	assert.False(t, sess.Conditions.Has("wounded"), "BUG-107: permanent condition must be cleared after respawn")
	assert.Equal(t, spawnRoomID, sess.RoomID, "player must be in spawn room after respawn")
}

// TestRespawnPlayer_RestoresSpontaneousUsePools verifies that spontaneous use pools
// are restored to Max on respawn (BUG-107).
//
// Precondition: pool at level 1 has Remaining=0, Max=3.
// Postcondition: sess.SpontaneousUsePools[1].Remaining == 3.
func TestRespawnPlayer_RestoresSpontaneousUsePools(t *testing.T) {
	wMgr, spawnRoomID := newBug107World()
	sMgr := session.NewManager()
	npcMgr := npc.NewManager()
	worldH := NewWorldHandler(wMgr, sMgr, npcMgr, nil, nil, &inventory.Registry{})
	logger := zap.NewNop()

	sess, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "uid-bug107b",
		Username:    "tester",
		CharName:    "Hero",
		CharacterID: 1,
		RoomID:      spawnRoomID,
		CurrentHP:   0,
		MaxHP:       20,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)

	sess.Dead = true
	sess.Conditions = condition.NewActiveSet()
	// Simulate a spent spontaneous pool.
	sess.SpontaneousUsePools = map[int]session.UsePool{
		1: {Remaining: 0, Max: 3},
	}

	poolRepo := newStubSpontaneousUsePoolRepo(map[int]session.UsePool{
		1: {Remaining: 0, Max: 3},
	})

	svc := &GameServiceServer{
		world:                  wMgr,
		sessions:               sMgr,
		worldH:                 worldH,
		logger:                 logger,
		spontaneousUsePoolRepo: poolRepo,
	}

	svc.respawnPlayer("uid-bug107b")

	require.Contains(t, sess.SpontaneousUsePools, 1, "pool for level 1 must exist after respawn")
	assert.Equal(t, 3, sess.SpontaneousUsePools[1].Remaining,
		"BUG-107: spontaneous use pool Remaining must equal Max after respawn")
}

// TestRespawnPlayer_RestoresFocusPoints verifies that focus points are restored to max on respawn.
//
// Precondition: sess.FocusPoints=0, sess.MaxFocusPoints=2.
// Postcondition: sess.FocusPoints==2.
func TestRespawnPlayer_RestoresFocusPoints(t *testing.T) {
	wMgr, spawnRoomID := newBug107World()
	sMgr := session.NewManager()
	npcMgr := npc.NewManager()
	worldH := NewWorldHandler(wMgr, sMgr, npcMgr, nil, nil, &inventory.Registry{})
	logger := zap.NewNop()

	sess, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "uid-bug107c",
		Username:    "tester",
		CharName:    "Hero",
		CharacterID: 0,
		RoomID:      spawnRoomID,
		CurrentHP:   0,
		MaxHP:       15,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)

	sess.Dead = true
	sess.Conditions = condition.NewActiveSet()
	sess.FocusPoints = 0
	sess.MaxFocusPoints = 2

	svc := &GameServiceServer{
		world:    wMgr,
		sessions: sMgr,
		worldH:   worldH,
		logger:   logger,
	}

	svc.respawnPlayer("uid-bug107c")

	assert.Equal(t, 2, sess.FocusPoints,
		"BUG-107: FocusPoints must equal MaxFocusPoints after respawn")
}

// TestRespawnPlayer_ClearsAllConditions verifies that ClearAll removes every condition
// type (encounter, permanent, rounds, until_save) on respawn (BUG-107).
//
// Precondition: ActiveSet has one condition of each duration type.
// Postcondition: ActiveSet is empty after respawnPlayer.
func TestRespawnPlayer_ClearsAllConditions(t *testing.T) {
	wMgr, spawnRoomID := newBug107World()
	sMgr := session.NewManager()
	npcMgr := npc.NewManager()
	worldH := NewWorldHandler(wMgr, sMgr, npcMgr, nil, nil, &inventory.Registry{})
	logger := zap.NewNop()

	sess, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "uid-bug107d",
		Username:    "tester",
		CharName:    "Hero",
		CharacterID: 0,
		RoomID:      spawnRoomID,
		CurrentHP:   0,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)

	sess.Dead = true
	sess.Conditions = condition.NewActiveSet()
	uid := "uid-bug107d"
	defs := []*condition.ConditionDef{
		{ID: "surge", Name: "Surge", DurationType: "encounter", MaxStacks: 0},
		{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0},
		{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4},
		{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4},
	}
	for _, def := range defs {
		dur := -1
		if def.DurationType == "rounds" {
			dur = 3
		}
		require.NoError(t, sess.Conditions.Apply(uid, def, 1, dur))
	}
	require.Len(t, sess.Conditions.All(), 4, "pre-condition: all 4 conditions must be active before respawn")

	svc := &GameServiceServer{
		world:    wMgr,
		sessions: sMgr,
		worldH:   worldH,
		logger:   logger,
	}

	svc.respawnPlayer(uid)

	assert.Empty(t, sess.Conditions.All(),
		"BUG-107: all conditions must be cleared after respawn, not just encounter-type")
}
