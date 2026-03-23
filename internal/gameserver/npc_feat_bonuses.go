package gameserver

import (
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

// NPCEffectiveStats holds combat bonuses computed from an NPC's passive feats.
type NPCEffectiveStats struct {
	// AttackBonus is the bonus to attack rolls from passive feats (e.g. pack_tactics).
	AttackBonus int
	// DamageBonus is the bonus to damage rolls from passive feats (e.g. brutal_strike).
	DamageBonus int
	// ACBonus is the bonus to AC from passive feats (e.g. evasive).
	ACBonus int
}

// ComputeNPCEffectiveStats returns the combat stat bonuses from the NPC's passive feats.
// roomNPCs is the list of all NPC instances in the room (used for pack_tactics evaluation).
// Pass nil for roomNPCs when not in combat or when the list is unavailable.
//
// Precondition: inst must not be nil; registry must not be nil.
// Postcondition: Returns an NPCEffectiveStats with summed bonuses from all passive feats.
func ComputeNPCEffectiveStats(inst *npc.Instance, registry *ruleset.FeatRegistry, roomNPCs []*npc.Instance) NPCEffectiveStats {
	var stats NPCEffectiveStats
	for _, featID := range inst.Feats {
		f, ok := registry.Feat(featID)
		if !ok || !f.AllowNPC {
			continue
		}
		switch featID {
		case "brutal_strike":
			stats.DamageBonus += 2
		case "evasive":
			stats.ACBonus += 2
		case "pack_tactics":
			for _, ally := range roomNPCs {
				if ally.ID != inst.ID {
					stats.AttackBonus += 2
					break
				}
			}
		// "tough" is applied at spawn, not at round resolution — skip here.
		}
	}
	return stats
}
