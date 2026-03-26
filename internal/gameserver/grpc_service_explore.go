package gameserver

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/danger"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/skillcheck"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// applyExploreModeOnEntry fires the room-entry exploration hook for the player's
// active mode. Called from handleMove after the room description has been sent.
//
// Precondition: uid, sess, and room must not be nil; sess.ExploreMode is set.
// Postcondition: Returns console messages to send to the player; sess state may be mutated.
func (s *GameServiceServer) applyExploreModeOnEntry(uid string, sess *session.PlayerSession, room *world.Room) []string {
	switch sess.ExploreMode {
	case session.ExploreModeLayLow:
		return s.exploreLayLow(uid, sess, room)
	case session.ExploreModeActiveSensors:
		return s.exploreActiveSensors(uid, sess, room)
	case session.ExploreModeCaseIt:
		return s.exploreCaseIt(uid, sess, room)
	case session.ExploreModePokeAround:
		return s.explorePokeAround(uid, sess, room)
	case session.ExploreModeShadow:
		return s.exploreShadow(uid, sess, room)
	}
	return nil
}

// ApplyExploreModeOnEntry is the exported wrapper used by tests.
func (s *GameServiceServer) ApplyExploreModeOnEntry(uid string, sess *session.PlayerSession, room *world.Room) []string {
	return s.applyExploreModeOnEntry(uid, sess, room)
}

// exploreDangerDC returns the skill check DC for the given room.
// Unset danger level defaults to 16 (Sketchy) per REQ-EXP-18 and REQ-EXP-23.
func (s *GameServiceServer) exploreDangerDC(room *world.Room) int {
	var zoneDangerLevel string
	if zone, ok := s.world.GetZone(room.ZoneID); ok {
		zoneDangerLevel = zone.DangerLevel
	}
	effective := danger.EffectiveDangerLevel(zoneDangerLevel, room.DangerLevel)
	switch effective {
	case danger.Safe:
		return 12
	case danger.Sketchy:
		return 16
	case danger.Dangerous:
		return 20
	case danger.AllOutWar:
		return 24
	default:
		return 16
	}
}

// exploreRoll performs a d20 secret check and returns the outcome.
func (s *GameServiceServer) exploreRoll(sess *session.PlayerSession, skill string, dc int) skillcheck.CheckOutcome {
	var roll int
	if s.dice != nil {
		roll = s.dice.Src().Intn(20) + 1
	} else {
		roll = 10
	}
	abilityScore := s.abilityScoreForSkill(sess, skill)
	amod := abilityModFrom(abilityScore)
	rank := ""
	if sess.Skills != nil {
		rank = sess.Skills[skill]
	}
	result := skillcheck.Resolve(roll, amod, rank, dc, skillcheck.TriggerDef{})
	return result.Outcome
}

// exploreLayLow handles the Lay Low mode room-entry hook (REQ-EXP-7 through REQ-EXP-10).
func (s *GameServiceServer) exploreLayLow(uid string, sess *session.PlayerSession, room *world.Room) []string {
	insts := s.npcMgr.InstancesInRoom(room.ID)
	var liveNPCs []*npc.Instance
	for _, inst := range insts {
		if !inst.IsDead() {
			liveNPCs = append(liveNPCs, inst)
		}
	}
	if len(liveNPCs) == 0 {
		return nil // REQ-EXP-10: no check when no NPCs
	}

	maxAwareness := 0
	for _, inst := range liveNPCs {
		if inst.Awareness > maxAwareness {
			maxAwareness = inst.Awareness
		}
	}
	dc := 10 + maxAwareness

	outcome := s.exploreRoll(sess, "ghosting", dc)
	switch outcome {
	case skillcheck.CritSuccess:
		s.applyExploreCondition(uid, sess, "hidden")
		s.applyExploreCondition(uid, sess, "undetected")
		return []string{"You slip into the shadows completely unnoticed."}
	case skillcheck.Success:
		s.applyExploreCondition(uid, sess, "hidden")
		return []string{"You blend into the background, staying hidden."}
	case skillcheck.CritFailure:
		sess.LayLowBlockedRoom = room.ID // REQ-EXP-8a
		return []string{"You fumble your attempt at stealth — NPCs in the area are on alert."}
	default:
		return nil // REQ-EXP-8a: failure has no effect
	}
}

// exploreActiveSensors handles Active Sensors mode (REQ-EXP-14 through REQ-EXP-18).
func (s *GameServiceServer) exploreActiveSensors(uid string, sess *session.PlayerSession, room *world.Room) []string {
	dc := s.exploreDangerDC(room)
	outcome := s.exploreRoll(sess, "tech_lore", dc)

	switch outcome {
	case skillcheck.CritSuccess, skillcheck.Success:
		var items []string
		for _, slot := range room.Equipment {
			if slot.ItemID != "" {
				items = append(items, slot.ItemID)
			}
		}
		for _, inst := range s.npcMgr.InstancesInRoom(room.ID) {
			if !inst.IsDead() && (inst.Type == "robot" || inst.Type == "machine") {
				items = append(items, inst.Name())
			}
		}
		if outcome == skillcheck.CritSuccess {
			items = append(items, s.concealedTechInRoom(room)...)
		}
		if len(items) == 0 {
			return []string{"Your sensors detect no active technology in this area."}
		}
		return []string{fmt.Sprintf("Your active sensors detect: %s.", strings.Join(items, ", "))}
	default:
		return nil // REQ-EXP-17: failure reveals nothing
	}
}

// concealedTechInRoom returns concealed tech using RoomTrapConfig.TemplateID.
func (s *GameServiceServer) concealedTechInRoom(room *world.Room) []string {
	var out []string
	for _, trap := range room.Traps {
		if trap.TemplateID != "" {
			out = append(out, fmt.Sprintf("concealed trap (%s)", trap.TemplateID))
		}
	}
	return out
}

// exploreCaseIt handles Case It mode (REQ-EXP-19 through REQ-EXP-24).
func (s *GameServiceServer) exploreCaseIt(uid string, sess *session.PlayerSession, room *world.Room) []string {
	dc := s.exploreDangerDC(room)
	outcome := s.exploreRoll(sess, "awareness", dc)

	switch outcome {
	case skillcheck.CritSuccess, skillcheck.Success:
		var findings []string
		for _, exit := range room.Exits {
			if exit.Hidden {
				findings = append(findings, fmt.Sprintf("hidden exit (%s)", exit.Direction))
			}
		}
		for _, trap := range room.Traps {
			entry := fmt.Sprintf("trap at %s", trap.Position)
			if outcome == skillcheck.CritSuccess {
				entry = fmt.Sprintf("trap: %s at %s", trap.TemplateID, trap.Position)
			}
			findings = append(findings, entry)
		}
		if len(findings) == 0 {
			return []string{"You look around carefully but find nothing out of the ordinary."}
		}
		return []string{fmt.Sprintf("You notice: %s.", strings.Join(findings, "; "))}
	default:
		return nil // REQ-EXP-22: failure reveals nothing
	}
}

// explorePokeAround handles Poke Around mode (REQ-EXP-33 through REQ-EXP-37).
func (s *GameServiceServer) explorePokeAround(uid string, sess *session.PlayerSession, room *world.Room) []string {
	skill, dc := pokeAroundSkillAndDC(sess, room)
	outcome := s.exploreRoll(sess, skill, dc)
	facts := s.loreFacts(room, outcome)
	if len(facts) == 0 {
		return nil
	}
	return facts
}

// pokeAroundSkillAndDC selects skill and DC based on room context (REQ-EXP-34).
func pokeAroundSkillAndDC(sess *session.PlayerSession, room *world.Room) (string, int) {
	ctx := ""
	if room.Properties != nil {
		ctx = room.Properties["context"]
	}
	switch ctx {
	case "history":
		return "intel", 15
	case "faction":
		conspRank := skillRankBonus(sess.Skills["conspiracy"])
		factRank := skillRankBonus(sess.Skills["factions"])
		if factRank > conspRank {
			return "factions", 17
		}
		return "conspiracy", 17
	case "technology":
		return "tech_lore", 16
	case "creature":
		return "wasteland", 14
	default:
		return "intel", 15
	}
}

// loreFacts returns lore strings for the room (REQ-EXP-35, REQ-EXP-36).
func (s *GameServiceServer) loreFacts(room *world.Room, outcome skillcheck.CheckOutcome) []string {
	if outcome == skillcheck.Failure || outcome == skillcheck.CritFailure {
		return nil
	}
	facts := room.LoreFacts()
	if len(facts) == 0 {
		return nil
	}
	if outcome == skillcheck.CritSuccess && len(facts) >= 2 {
		return facts[:2]
	}
	return facts[:1]
}

// exploreShadow handles Shadow mode validation (REQ-EXP-31, REQ-EXP-32).
// Rank override for room skill checks occurs in applyRoomSkillChecks (Task 6).
func (s *GameServiceServer) exploreShadow(uid string, sess *session.PlayerSession, room *world.Room) []string {
	ally := s.sessions.GetPlayerByCharID(sess.ExploreShadowTarget)
	if ally == nil || ally.RoomID != sess.RoomID {
		return nil // REQ-EXP-31: suspend silently
	}
	return nil
}

// applyExploreCondition applies a named condition, silently skipping if not found.
func (s *GameServiceServer) applyExploreCondition(uid string, sess *session.PlayerSession, condID string) {
	if s.condRegistry == nil {
		return
	}
	def, ok := s.condRegistry.Get(condID)
	if !ok {
		return
	}
	if sess.Conditions == nil {
		return
	}
	_ = sess.Conditions.Apply(uid, def, 1, -1)
}

// applyExploreModeOnCombatStart fires the combat-start exploration hook for the player.
// Called after RollInitiative in combat_handler.go before StartCombat.
//
// Precondition: sess, playerCbt, and h must not be nil.
// Postcondition: Lay Low is cleared; Hold Ground applies shield_raised + ACMod;
//
//	Run Point applies +1 Initiative to co-located players.
func applyExploreModeOnCombatStart(sess *session.PlayerSession, playerCbt *combat.Combatant, h *CombatHandler) []string {
	var msgs []string

	// Lay Low: clear before other hooks (REQ-EXP-40).
	if sess.ExploreMode == session.ExploreModeLayLow {
		sess.ExploreMode = ""
		// No message — mode clears silently at combat start.
	}

	// Hold Ground: apply shield_raised at no AP cost (REQ-EXP-11, REQ-EXP-12).
	if sess.ExploreMode == session.ExploreModeHoldGround {
		if hasShieldEquipped(sess) {
			// Apply condition.
			if h.condRegistry != nil {
				if def, ok := h.condRegistry.Get("shield_raised"); ok {
					if sess.Conditions != nil {
						_ = sess.Conditions.Apply(sess.UID, def, 1, -1)
					}
				}
			}
			// Apply +2 ACMod to combatant.
			playerCbt.ACMod += 2
			msgs = append(msgs, "Hold Ground: your shield is already raised.")
		}
		// No error if no shield (REQ-EXP-12).
	}

	// Run Point: +1 circumstance bonus to Initiative for all other players in room (REQ-EXP-25, REQ-EXP-26).
	if sess.ExploreMode == session.ExploreModeRunPoint {
		others := h.sessions.PlayersInRoomDetails(sess.RoomID)
		for _, other := range others {
			if other.UID == sess.UID {
				continue // REQ-EXP-26: Run Point player does not receive the bonus.
			}
			// Adjust the combatant's initiative by +1 if they are already enrolled in this combat.
			if h.engine != nil {
				if cbt, ok := h.engine.GetCombat(sess.RoomID); ok {
					if otherCbt := cbt.GetCombatant(other.UID); otherCbt != nil {
						otherCbt.Initiative += 1
					}
				}
			}
		}
		msgs = append(msgs, "Run Point: your allies gain +1 to Initiative.")
	}

	return msgs
}

// hasShieldEquipped returns true if the player has a shield in the off-hand slot.
func hasShieldEquipped(sess *session.PlayerSession) bool {
	if sess.LoadoutSet == nil {
		return false
	}
	preset := sess.LoadoutSet.ActivePreset()
	return preset != nil && preset.OffHand != nil && preset.OffHand.Def != nil && preset.OffHand.Def.IsShield()
}

// ApplyExploreModeOnCombatStartForTest is an exported test shim.
func ApplyExploreModeOnCombatStartForTest(sess *session.PlayerSession, playerCbt *combat.Combatant, h *CombatHandler) []string {
	return applyExploreModeOnCombatStart(sess, playerCbt, h)
}
