package gameserver

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/dice"
)

// makeAggressiveNPCDomain returns an ai.Domain whose sole method always plans
// an "attack" action, simulating an NPC that always attacks when it can.
//
// Postcondition: Returns a non-nil Domain ready for ai.Registry.Register.
func makeAggressiveNPCDomain(domainID string) *ai.Domain {
	return &ai.Domain{
		ID:    domainID,
		Tasks: []*ai.Task{{ID: "behave"}},
		Methods: []*ai.Method{
			{TaskID: "behave", ID: "m_attack", Subtasks: []string{"do_attack"}},
		},
		Operators: []*ai.Operator{
			{ID: "do_attack", Action: "attack"},
		},
	}
}

// makePropFleeHandler builds a two-room world with adjacent exits for flee tests.
//
// Postcondition: Returns a non-nil CombatHandler and a world.Manager with rooms
// "prop-room-a" and "prop-room-b" connected bidirectionally.
func makePropFleeHandler(t *testing.T) (*CombatHandler, *world.Manager) {
	t.Helper()
	roomA := &world.Room{
		ID: "prop-room-a", ZoneID: "prop-zone",
		Exits: []world.Exit{{Direction: "north", TargetRoom: "prop-room-b"}},
	}
	roomB := &world.Room{
		ID: "prop-room-b", ZoneID: "prop-zone",
		Exits: []world.Exit{{Direction: "south", TargetRoom: "prop-room-a"}},
	}
	zone := &world.Zone{
		ID: "prop-zone", StartRoom: "prop-room-a",
		Rooms: map[string]*world.Room{"prop-room-a": roomA, "prop-room-b": roomB},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	if err != nil {
		t.Fatalf("makePropFleeHandler world.NewManager: %v", err)
	}
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, zap.NewNop())
	engine := combat.NewEngine()
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	h := NewCombatHandler(
		engine, npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration,
		makeTestConditionRegistry(),
		wm, nil, nil, nil, nil, nil, nil,
	)
	return h, wm
}

// TestProperty_InitiateNPCCombat_AgressiveNPCEngages is a property-based test
// that verifies: for any NPC with CourageThreshold >= 0 and a player in the
// same room, InitiateNPCCombat starts an active combat in that room.
//
// REQ-NB-7.
func TestProperty_InitiateNPCCombat_AggressiveNPCEngages(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roomID := fmt.Sprintf("prop-initiate-room-%d", rapid.IntRange(1, 1000).Draw(rt, "room_suffix"))
		playerUID := fmt.Sprintf("prop-player-%d", rapid.IntRange(1, 1000).Draw(rt, "player_suffix"))
		npcLevel := rapid.IntRange(1, 20).Draw(rt, "npc_level")
		npcHP := rapid.IntRange(1, 200).Draw(rt, "npc_hp")
		courageThreshold := rapid.IntRange(0, 100).Draw(rt, "courage_threshold")

		src := dice.NewCryptoSource()
		roller := dice.NewLoggedRoller(src, zap.NewNop())
		engine := combat.NewEngine()
		npcMgr := npc.NewManager()
		sessMgr := session.NewManager()
		h := NewCombatHandler(
			engine, npcMgr, sessMgr, roller,
			func(_ string, _ []*gamev1.CombatEvent) {},
			testRoundDuration,
			makeTestConditionRegistry(),
			nil, nil, nil, nil, nil, nil, nil,
		)

		tmpl := &npc.Template{
			ID:    "prop-aggro-npc",
			Name:  "PropAggroNPC",
			Level: npcLevel,
			MaxHP: npcHP,
			AC:    12,
		}
		inst, err := npcMgr.Spawn(tmpl, roomID)
		if err != nil {
			rt.Fatalf("Spawn: %v", err)
		}
		inst.CourageThreshold = courageThreshold

		// Register a player in the room.
		_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:      playerUID,
			Username: "propuser",
			CharName: "PropHero",
			RoomID:   roomID,
			CurrentHP: 10,
			MaxHP:     10,
			Role:      "player",
		})
		if err != nil {
			rt.Fatalf("AddPlayer: %v", err)
		}

		// InitiateNPCCombat should start combat between inst and playerUID.
		h.InitiateNPCCombat(inst, playerUID)
		defer h.cancelTimer(roomID)

		h.combatMu.RLock()
		_, active := h.engine.GetCombat(roomID)
		h.combatMu.RUnlock()

		// Property: any NPC initiating against a valid player in same room starts combat.
		if !active {
			rt.Fatalf("expected active combat after InitiateNPCCombat (level=%d hp=%d threshold=%d)",
				npcLevel, npcHP, courageThreshold)
		}
	})
}

// TestProperty_Flee_NPCRemovedFromCombatants is a property-based test verifying
// that after a flee operator executes, the NPC is no longer an active (living)
// combatant in the combat engine.
//
// REQ-NB-25.
func TestProperty_Flee_NPCRemovedFromCombatants(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Use fixed room IDs matching makePropFleeHandler so worldMgr.GetRoom succeeds.
		const roomID = "prop-room-a"
		npcHP := rapid.IntRange(5, 200).Draw(rt, "npc_hp") // >= 5 to survive one combat round
		npcLevel := rapid.IntRange(1, 20).Draw(rt, "npc_level")
		playerSuffix := rapid.IntRange(1, 10000).Draw(rt, "player_suffix")
		playerUID := fmt.Sprintf("prop-flee-player-%d", playerSuffix)

		h, _ := makePropFleeHandler(t)

		tmpl := &npc.Template{
			ID:    fmt.Sprintf("prop-flee-npc-%d-%d", npcLevel, playerSuffix),
			Name:  fmt.Sprintf("PropFleeNPC%d", playerSuffix),
			Level: npcLevel,
			MaxHP: npcHP,
			AC:    15,
		}
		inst, err := h.npcMgr.Spawn(tmpl, roomID)
		if err != nil {
			rt.Fatalf("Spawn: %v", err)
		}

		_, err = h.sessions.AddPlayer(session.AddPlayerOptions{
			UID:       playerUID,
			Username:  "propuser",
			CharName:  fmt.Sprintf("PropHero%d", playerSuffix),
			RoomID:    roomID,
			CurrentHP: 10,
			MaxHP:     10,
			Role:      "player",
		})
		if err != nil {
			rt.Fatalf("AddPlayer: %v", err)
		}

		_, err = h.Attack(playerUID, inst.Name())
		if err != nil {
			rt.Fatalf("Attack: %v", err)
		}
		defer h.cancelTimer(roomID)

		plan := []ai.PlannedAction{{Action: "flee", OperatorID: "__prop_flee"}}
		h.combatMu.Lock()
		cbt, ok := h.engine.GetCombat(roomID)
		if ok {
			actor := cbt.GetCombatant(inst.ID)
			if actor != nil {
				h.applyPlanLocked(cbt, actor, plan)
			}
		}
		h.combatMu.Unlock()

		// Property: after flee, the NPC must not be a living combatant in any active combat.
		// Combat may have ended entirely (ok2=false) — that also satisfies the property.
		h.combatMu.RLock()
		cbt2, ok2 := h.engine.GetCombat(roomID)
		var foundLiving bool
		if ok2 && cbt2 != nil {
			for _, c := range cbt2.Combatants {
				if c.ID == inst.ID && !c.IsDead() {
					foundLiving = true
					break
				}
			}
		}
		h.combatMu.RUnlock()

		if foundLiving {
			rt.Fatalf("NPC still a living combatant after flee (hp=%d level=%d)", npcHP, npcLevel)
		}
	})
}

// TestProperty_TargetWeakest_TargetsLowestHP is a property-based test that
// verifies: when 2+ players are in combat, target_weakest always queues an
// attack targeting the player with the lowest current HP percentage.
//
// REQ-NB-29.
func TestProperty_TargetWeakest_TargetsLowestHP(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		const roomID = "prop-tw-room-a"

		// Generate two players with distinct HP percentages.
		hp1 := rapid.IntRange(1, 100).Draw(rt, "hp1")
		hp2 := rapid.IntRange(1, 100).Draw(rt, "hp2")
		// Ensure they are not equal so there is a clear weakest.
		if hp1 == hp2 {
			hp2++
		}

		p1UID := fmt.Sprintf("prop-tw-p1-%d-%d", hp1, hp2)
		p2UID := fmt.Sprintf("prop-tw-p2-%d-%d", hp1, hp2)

		h, _ := makePropFleeHandler(t)

		tmpl := &npc.Template{
			ID:    "prop-tw-npc",
			Name:  "PropTWNPC",
			Level: 1,
			MaxHP: 50,
			AC:    12,
		}
		inst, err := h.npcMgr.Spawn(tmpl, roomID)
		if err != nil {
			rt.Fatalf("Spawn: %v", err)
		}

		// Add player 1 (attacker used to start combat).
		sess1, err := h.sessions.AddPlayer(session.AddPlayerOptions{
			UID:       p1UID,
			Username:  "propuser1",
			CharName:  fmt.Sprintf("PropP1-%d", hp1),
			RoomID:    roomID,
			CurrentHP: hp1,
			MaxHP:     100,
			Role:      "player",
		})
		if err != nil {
			rt.Fatalf("AddPlayer p1: %v", err)
		}

		// Add player 2.
		sess2, err := h.sessions.AddPlayer(session.AddPlayerOptions{
			UID:       p2UID,
			Username:  "propuser2",
			CharName:  fmt.Sprintf("PropP2-%d", hp2),
			RoomID:    roomID,
			CurrentHP: hp2,
			MaxHP:     100,
			Role:      "player",
		})
		if err != nil {
			rt.Fatalf("AddPlayer p2: %v", err)
		}

		_, err = h.Attack(p1UID, inst.Name())
		if err != nil {
			rt.Fatalf("Attack: %v", err)
		}
		defer h.cancelTimer(roomID)

		// Add p2 as a combatant.
		h.combatMu.Lock()
		_, ok := h.engine.GetCombat(roomID)
		if ok {
			_ = h.engine.AddCombatant(roomID, &combat.Combatant{
				ID:        sess2.UID,
				Name:      sess2.CharName,
				Kind:      combat.KindPlayer,
				CurrentHP: sess2.CurrentHP,
				MaxHP:     sess2.MaxHP,
				AC:        12,
				Level:     1,
			})
		}
		h.combatMu.Unlock()

		if !ok {
			rt.Skip()
			return
		}

		plan := []ai.PlannedAction{{Action: "target_weakest", OperatorID: "__prop_tw"}}
		h.combatMu.Lock()
		cbt, cbtOK := h.engine.GetCombat(roomID)
		if cbtOK {
			actor := cbt.GetCombatant(inst.ID)
			if actor != nil {
				h.applyPlanLocked(cbt, actor, plan)
			}
		}
		h.combatMu.Unlock()

		// Property: the queued attack targets the lower-HP player.
		var weakestName string
		if hp1 <= hp2 {
			weakestName = sess1.CharName
		} else {
			weakestName = sess2.CharName
		}

		h.combatMu.RLock()
		cbt2, _ := h.engine.GetCombat(roomID)
		var foundWeakest bool
		if cbt2 != nil {
			if aq, found := cbt2.ActionQueues[inst.ID]; found {
				for _, qa := range aq.QueuedActions() {
					if qa.Type == combat.ActionAttack && qa.Target == weakestName {
						foundWeakest = true
						break
					}
				}
			}
		}
		h.combatMu.RUnlock()

		if !foundWeakest {
			rt.Fatalf("expected queued attack targeting weakest player %q (hp1=%d hp2=%d)",
				weakestName, hp1, hp2)
		}
	})
}
