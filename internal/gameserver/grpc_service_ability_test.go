package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap"
)

// makeAbilityCombatHandler constructs a CombatHandler with mentalStateMgr wired.
//
// Precondition: t is non-nil; mentalMgr may be nil.
// Postcondition: Returns a non-nil CombatHandler with mentalStateMgr set.
func makeAbilityCombatHandler(t *testing.T, broadcastFn func(string, []*gamev1.CombatEvent), mentalMgr *mentalstate.Manager) *CombatHandler {
	t.Helper()
	_ = zap.NewNop()
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, zap.NewNop())
	engine := combat.NewEngine()
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	aiReg := ai.NewRegistry()
	return NewCombatHandler(
		engine,
		npcMgr,
		sessMgr,
		roller,
		broadcastFn,
		testRoundDuration,
		makeTestConditionRegistry(),
		nil, // worldMgr
		nil, // scriptMgr
		nil, // invRegistry
		aiReg,
		nil, // respawnMgr
		nil, // floorMgr
		mentalMgr,
	)
}

// spawnAbilityTestNPC creates and registers an NPC instance with pre-set fields.
//
// Precondition: npcMgr non-nil; cooldowns may be nil.
// Postcondition: Returns a non-nil Instance registered with npcMgr.
func spawnAbilityTestNPC(t *testing.T, npcMgr *npc.Manager, roomID string, cooldowns map[string]int) *npc.Instance {
	t.Helper()
	tmpl := &npc.Template{
		ID:    "ability-ganger",
		Name:  "Ganger",
		Level: 1,
		MaxHP: 20,
		AC:    13,
	}
	inst, err := npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatalf("spawnAbilityTestNPC: %v", err)
	}
	if cooldowns != nil {
		inst.AbilityCooldowns = cooldowns
	}
	return inst
}

// startAbilityCombat creates a combat via Engine.StartCombat in h.engine.
//
// Precondition: all combatants have Initiative set.
// Postcondition: Returns a non-nil *combat.Combat registered in h.engine.
func startAbilityCombat(t *testing.T, h *CombatHandler, roomID string, combatants []*combat.Combatant) *combat.Combat {
	t.Helper()
	cbt, err := h.engine.StartCombat(roomID, combatants, makeTestConditionRegistry(), nil, "")
	if err != nil {
		t.Fatalf("startAbilityCombat: %v", err)
	}
	return cbt
}

// TestApplyPlanLocked_ApplyMentalState_OnCooldown verifies that apply_mental_state
// is skipped without error when the operator is still on cooldown.
//
// Precondition: NPC AbilityCooldowns["ganger_taunt"] == 2.
// Postcondition: mentalstate track severity remains SeverityNone; cooldown unchanged.
func TestApplyPlanLocked_ApplyMentalState_OnCooldown(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	h := makeAbilityCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {}, mentalMgr)

	const roomID = "room-ability-1"
	const playerUID = "player-cd-1"

	inst := spawnAbilityTestNPC(t, h.npcMgr, roomID, map[string]int{"ganger_taunt": 2})
	addTestPlayer(t, h.sessions, playerUID, roomID)

	npcCbt := &combat.Combatant{
		ID:         inst.ID,
		Name:       inst.Name(),
		Kind:       combat.KindNPC,
		CurrentHP:  20,
		MaxHP:      20,
		AC:         13,
		Level:      1,
		Initiative: 5,
	}
	playerCbt := &combat.Combatant{
		ID:         playerUID,
		Name:       "Hero",
		Kind:       combat.KindPlayer,
		CurrentHP:  10,
		MaxHP:      10,
		AC:         14,
		Level:      1,
		Initiative: 10,
	}

	cbt := startAbilityCombat(t, h, roomID, []*combat.Combatant{npcCbt, playerCbt})

	actions := []ai.PlannedAction{
		{
			Action:         "apply_mental_state",
			Target:         "nearest_enemy",
			OperatorID:     "ganger_taunt",
			Track:          "rage",
			Severity:       "mild",
			CooldownRounds: 3,
			APCost:         1,
		},
	}

	h.combatMu.Lock()
	h.applyPlanLocked(cbt, npcCbt, actions)
	h.combatMu.Unlock()

	// Mental state must NOT have been applied.
	if got := mentalMgr.CurrentSeverity(playerUID, mentalstate.TrackRage); got != mentalstate.SeverityNone {
		t.Errorf("expected SeverityNone; got %v", got)
	}

	// Cooldown must remain 2 (not zeroed or modified).
	if got := inst.AbilityCooldowns["ganger_taunt"]; got != 2 {
		t.Errorf("expected cooldown 2; got %d", got)
	}
}

// TestApplyPlanLocked_ApplyMentalState_Execute verifies that apply_mental_state
// applies the mental state and sets the cooldown when the operator is off cooldown.
//
// Precondition: NPC has no cooldown for "ganger_taunt".
// Postcondition: rage track is SeverityMild for target; cooldown set to 3.
func TestApplyPlanLocked_ApplyMentalState_Execute(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	h := makeAbilityCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {}, mentalMgr)

	const roomID = "room-ability-2"
	const playerUID = "player-exec-1"

	inst := spawnAbilityTestNPC(t, h.npcMgr, roomID, nil)
	addTestPlayer(t, h.sessions, playerUID, roomID)

	npcCbt := &combat.Combatant{
		ID:         inst.ID,
		Name:       inst.Name(),
		Kind:       combat.KindNPC,
		CurrentHP:  20,
		MaxHP:      20,
		AC:         13,
		Level:      1,
		Initiative: 5,
	}
	playerCbt := &combat.Combatant{
		ID:         playerUID,
		Name:       "Hero",
		Kind:       combat.KindPlayer,
		CurrentHP:  10,
		MaxHP:      10,
		AC:         14,
		Level:      1,
		Initiative: 10,
	}

	cbt := startAbilityCombat(t, h, roomID, []*combat.Combatant{npcCbt, playerCbt})

	actions := []ai.PlannedAction{
		{
			Action:         "apply_mental_state",
			Target:         "nearest_enemy",
			OperatorID:     "ganger_taunt",
			Track:          "rage",
			Severity:       "mild",
			CooldownRounds: 3,
			APCost:         1,
		},
	}

	h.combatMu.Lock()
	h.applyPlanLocked(cbt, npcCbt, actions)
	h.combatMu.Unlock()

	// Mental state must have been applied.
	if got := mentalMgr.CurrentSeverity(playerUID, mentalstate.TrackRage); got != mentalstate.SeverityMild {
		t.Errorf("expected SeverityMild; got %v", got)
	}

	// Cooldown must be set to 3.
	if got := inst.AbilityCooldowns["ganger_taunt"]; got != 3 {
		t.Errorf("expected cooldown 3; got %d", got)
	}
}

// TestAutoQueueNPCsLocked_CooldownDecrement verifies that autoQueueNPCsLocked
// decrements positive cooldowns and floors at 0.
//
// Precondition: NPC AbilityCooldowns{"ganger_taunt": 2, "other_op": 0}.
// Postcondition: ganger_taunt == 1; other_op == 0.
func TestAutoQueueNPCsLocked_CooldownDecrement(t *testing.T) {
	h := makeAbilityCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {}, nil)

	const roomID = "room-ability-3"
	const playerUID = "player-dec-1"

	inst := spawnAbilityTestNPC(t, h.npcMgr, roomID, map[string]int{"ganger_taunt": 2, "other_op": 0})
	addTestPlayer(t, h.sessions, playerUID, roomID)

	npcCbt := &combat.Combatant{
		ID:         inst.ID,
		Name:       inst.Name(),
		Kind:       combat.KindNPC,
		CurrentHP:  20,
		MaxHP:      20,
		AC:         13,
		Level:      1,
		Initiative: 5,
	}
	playerCbt := &combat.Combatant{
		ID:         playerUID,
		Name:       "Hero",
		Kind:       combat.KindPlayer,
		CurrentHP:  10,
		MaxHP:      10,
		AC:         14,
		Level:      1,
		Initiative: 10,
	}

	cbt := startAbilityCombat(t, h, roomID, []*combat.Combatant{npcCbt, playerCbt})

	h.combatMu.Lock()
	h.autoQueueNPCsLocked(cbt)
	h.combatMu.Unlock()

	// ganger_taunt decremented from 2 to 1.
	if got := inst.AbilityCooldowns["ganger_taunt"]; got != 1 {
		t.Errorf("expected ganger_taunt == 1; got %d", got)
	}

	// other_op stays at 0 (floor).
	if got := inst.AbilityCooldowns["other_op"]; got != 0 {
		t.Errorf("expected other_op == 0; got %d", got)
	}
}

// TestApplyPlanLocked_TargetSelector_HighestDamage verifies that highest_damage_enemy
// selects the player who dealt the most damage.
//
// Precondition: p1 dealt 5 damage, p2 dealt 15 damage.
// Postcondition: p2 has rage SeverityMild; p1 has SeverityNone.
func TestApplyPlanLocked_TargetSelector_HighestDamage(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	h := makeAbilityCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {}, mentalMgr)

	const roomID = "room-ability-4"
	const p1UID = "player-hd-1"
	const p2UID = "player-hd-2"

	inst := spawnAbilityTestNPC(t, h.npcMgr, roomID, nil)
	addTestPlayer(t, h.sessions, p1UID, roomID)
	addTestPlayer(t, h.sessions, p2UID, roomID)

	npcCbt := &combat.Combatant{
		ID:         inst.ID,
		Name:       inst.Name(),
		Kind:       combat.KindNPC,
		CurrentHP:  20,
		MaxHP:      20,
		AC:         13,
		Level:      1,
		Initiative: 5,
	}
	p1Cbt := &combat.Combatant{
		ID:         p1UID,
		Name:       "Hero1",
		Kind:       combat.KindPlayer,
		CurrentHP:  10,
		MaxHP:      10,
		AC:         14,
		Level:      1,
		Initiative: 10,
	}
	p2Cbt := &combat.Combatant{
		ID:         p2UID,
		Name:       "Hero2",
		Kind:       combat.KindPlayer,
		CurrentHP:  10,
		MaxHP:      10,
		AC:         14,
		Level:      1,
		Initiative: 9,
	}

	cbt := startAbilityCombat(t, h, roomID, []*combat.Combatant{npcCbt, p1Cbt, p2Cbt})
	cbt.RecordDamage(p1UID, 5)
	cbt.RecordDamage(p2UID, 15)

	actions := []ai.PlannedAction{
		{
			Action:         "apply_mental_state",
			Target:         "highest_damage_enemy",
			OperatorID:     "ganger_taunt",
			Track:          "rage",
			Severity:       "mild",
			CooldownRounds: 3,
			APCost:         1,
		},
	}

	h.combatMu.Lock()
	h.applyPlanLocked(cbt, npcCbt, actions)
	h.combatMu.Unlock()

	// p2 has higher damage — must have rage applied.
	if got := mentalMgr.CurrentSeverity(p2UID, mentalstate.TrackRage); got != mentalstate.SeverityMild {
		t.Errorf("expected p2 rage == SeverityMild; got %v", got)
	}

	// p1 must be unaffected.
	if got := mentalMgr.CurrentSeverity(p1UID, mentalstate.TrackRage); got != mentalstate.SeverityNone {
		t.Errorf("expected p1 rage == SeverityNone; got %v", got)
	}
}
