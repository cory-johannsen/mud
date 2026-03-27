package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/npc"
)

// newSeduceTestService creates a minimal GameServiceServer and a player session for seduce tests.
//
// Precondition: none.
// Postcondition: Returns svc and a registered session UID; the player has flair="trained" by default.
func newSeduceTestService(t testing.TB) (*GameServiceServer, string) {
	t.Helper()
	const uid = "seduce_uid"
	sMgr := addPlayerToSession(uid, "room1")
	sess, ok := sMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Skills = map[string]string{"flair": "trained"}
	svc := &GameServiceServer{sessions: sMgr}
	return svc, uid
}

// spawnSeduceTestNPC creates a minimal npc.Instance for seduce tests using Spawn.
//
// Precondition: gender may be empty (genderless) or non-empty (gendered).
// savvyStr is used as the template ability savvy to set inst.Savvy.
// Postcondition: Returns a spawned *npc.Instance with given gender, savvy, and default disposition.
func spawnSeduceTestNPC(t testing.TB, gender string, savvy int) *npc.Instance {
	t.Helper()
	mgr := npc.NewManager()
	tmpl := &npc.Template{
		ID:     "test_npc",
		Name:   "Test NPC",
		Gender: gender,
		Abilities: npc.Abilities{
			Savvy: savvy,
		},
		Disposition: "neutral",
	}
	inst, err := mgr.Spawn(tmpl, "room1")
	require.NoError(t, err)
	return inst
}

// TestSeduce_NoGender_Rejected verifies that a genderless NPC cannot be seduced.
//
// Precondition: inst.Gender == ""; player has flair rank.
// Postcondition: message contains "cannot be seduced".
func TestSeduce_NoGender_Rejected(t *testing.T) {
	svc, uid := newSeduceTestService(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	inst := spawnSeduceTestNPC(t, "", 0)

	msg, err := svc.executeSeduce(sess, uid, inst)
	require.NoError(t, err)
	assert.Contains(t, msg, "cannot be seduced")
}

// TestSeduce_NoFlair_Rejected verifies that a player with no flair rank cannot seduce.
//
// Precondition: sess.Skills["flair"] == ""; inst.Gender != "".
// Postcondition: message contains "lack the charm".
func TestSeduce_NoFlair_Rejected(t *testing.T) {
	svc, uid := newSeduceTestService(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	delete(sess.Skills, "flair")
	inst := spawnSeduceTestNPC(t, "female", 0)

	msg, err := svc.executeSeduce(sess, uid, inst)
	require.NoError(t, err)
	assert.Contains(t, msg, "lack the charm")
}

// TestSeduce_AlreadyCharmed_Rejected verifies that an already-charmed NPC cannot be seduced again.
//
// Precondition: NPC already has "charmed" condition in seduceConditions; player has flair rank.
// Postcondition: message contains "already charmed".
func TestSeduce_AlreadyCharmed_Rejected(t *testing.T) {
	svc, uid := newSeduceTestService(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	inst := spawnSeduceTestNPC(t, "female", 0)

	// Pre-populate seduceConditions with charmed condition.
	charmedSet := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "charmed", Name: "Charmed", DurationType: "until_save"}
	_ = charmedSet.Apply(inst.ID, def, 1, -1)
	svc.seduceConditions = map[string]*condition.ActiveSet{
		inst.ID: charmedSet,
	}

	msg, err := svc.executeSeduce(sess, uid, inst)
	require.NoError(t, err)
	assert.Contains(t, msg, "already charmed")
}

// TestSeduce_PreviouslyRejected_Rejected verifies that a previously-rejecting NPC won't be seduced.
//
// Precondition: inst.SeductionRejected[uid] == true; player has flair rank.
// Postcondition: message contains "not interested".
func TestSeduce_PreviouslyRejected_Rejected(t *testing.T) {
	svc, uid := newSeduceTestService(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	inst := spawnSeduceTestNPC(t, "male", 0)
	inst.SeductionRejected = map[string]bool{uid: true}

	msg, err := svc.executeSeduce(sess, uid, inst)
	require.NoError(t, err)
	assert.Contains(t, msg, "not interested")
}

// TestSeduce_Success_NPCCharmed verifies that on player success (dice nil → fallback 10+bonus),
// with legendary flair (bonus 8) and NPC Savvy=0, the NPC gains the charmed condition.
//
// Precondition: svc.dice == nil; sess.Skills["flair"]="legendary" (bonus=8); inst.Savvy=0.
// Postcondition: message contains "charmed"; seduceConditions[inst.ID].Has("charmed") == true.
func TestSeduce_Success_NPCCharmed(t *testing.T) {
	svc, uid := newSeduceTestService(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Skills["flair"] = "legendary" // bonus = 8 → total = 10+8 = 18

	inst := spawnSeduceTestNPC(t, "female", 0) // NPC total = 10+0 = 10

	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{ID: "charmed", Name: "Charmed", DurationType: "until_save"})
	svc.condRegistry = condReg
	svc.seduceConditions = make(map[string]*condition.ActiveSet)

	msg, err := svc.executeSeduce(sess, uid, inst)
	require.NoError(t, err)
	assert.Contains(t, msg, "charmed")

	cs, ok := svc.seduceConditions[inst.ID]
	require.True(t, ok, "seduceConditions should have an entry for inst.ID")
	assert.True(t, cs.Has("charmed"), "charmed condition should be active on NPC")
}

// TestSeduce_Failure_NPCTurnsHostile verifies that when NPC savvy overpowers player flair,
// NPC disposition flips to hostile and SeductionRejected is set.
//
// Precondition: svc.dice == nil; sess.Skills["flair"]="trained" (bonus=2); inst.Savvy=10.
// Postcondition: inst.Disposition == "hostile"; inst.SeductionRejected[uid] == true.
func TestSeduce_Failure_NPCTurnsHostile(t *testing.T) {
	svc, uid := newSeduceTestService(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Skills["flair"] = "trained" // bonus = 2 → total = 10+2 = 12

	inst := spawnSeduceTestNPC(t, "female", 10) // NPC total = 10+10 = 20

	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{ID: "charmed", Name: "Charmed", DurationType: "until_save"})
	svc.condRegistry = condReg
	svc.seduceConditions = make(map[string]*condition.ActiveSet)

	msg, err := svc.executeSeduce(sess, uid, inst)
	require.NoError(t, err)
	assert.Contains(t, msg, "hostile")
	assert.Equal(t, "hostile", inst.Disposition)
	assert.True(t, inst.SeductionRejected[uid])
}

// TestProperty_Seduce_HighFlairAlwaysCharms is a property test verifying that when the player's
// flair rank bonus strictly exceeds NPC Savvy and dice is nil (fallback 10), the NPC is always charmed.
//
// Precondition: flairBonus > npcSavvy; dice == nil.
// Postcondition: NPC gains charmed condition; message contains "charmed".
func TestProperty_Seduce_HighFlairAlwaysCharms(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Choose NPC savvy in [0,4] and pick a flair rank whose bonus exceeds it.
		npcSavvy := rapid.IntRange(0, 4).Draw(rt, "npc_savvy")
		// Ranks and their bonuses: trained=2, expert=4, master=6, legendary=8
		type rankEntry struct {
			rank  string
			bonus int
		}
		ranks := []rankEntry{
			{"trained", 2},
			{"expert", 4},
			{"master", 6},
			{"legendary", 8},
		}
		// Filter to ranks where bonus > npcSavvy
		var qualifying []rankEntry
		for _, r := range ranks {
			if r.bonus > npcSavvy {
				qualifying = append(qualifying, r)
			}
		}
		if len(qualifying) == 0 {
			rt.Skip("no qualifying rank for this savvy")
		}
		idx := rapid.IntRange(0, len(qualifying)-1).Draw(rt, "rank_idx")
		chosen := qualifying[idx]

		svc, uid := newSeduceTestService(t)
		sess, ok := svc.sessions.GetPlayer(uid)
		require.True(t, ok)
		sess.Skills["flair"] = chosen.rank

		inst := spawnSeduceTestNPC(t, "female", npcSavvy)

		condReg := condition.NewRegistry()
		condReg.Register(&condition.ConditionDef{ID: "charmed", Name: "Charmed", DurationType: "until_save"})
		svc.condRegistry = condReg
		svc.seduceConditions = make(map[string]*condition.ActiveSet)

		msg, err := svc.executeSeduce(sess, uid, inst)
		require.NoError(rt, err)
		assert.Contains(rt, msg, "charmed")

		cs, ok := svc.seduceConditions[inst.ID]
		require.True(rt, ok, "seduceConditions must have entry for NPC")
		assert.True(rt, cs.Has("charmed"))
	})
}
