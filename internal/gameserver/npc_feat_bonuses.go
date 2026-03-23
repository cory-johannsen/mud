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
// TargetTags-gated feats are not evaluated; use ComputeNPCAttackStats when a target is available.
//
// Precondition: inst must not be nil; registry must not be nil.
// Postcondition: Returns an NPCEffectiveStats with summed bonuses from all passive feats.
func ComputeNPCEffectiveStats(inst *npc.Instance, registry *ruleset.FeatRegistry, roomNPCs []*npc.Instance) NPCEffectiveStats {
	return computeStats(inst, nil, registry, roomNPCs)
}

// ComputeNPCAttackStats returns the combat stat bonuses from the NPC's passive feats,
// evaluated against target. Feats with TargetTags only apply when target has at least
// one matching tag; pass nil target to skip all TargetTags-gated feats.
//
// Precondition: attacker must not be nil; registry must not be nil.
// Postcondition: Returns an NPCEffectiveStats with summed bonuses, respecting TargetTags.
func ComputeNPCAttackStats(attacker *npc.Instance, target *npc.Instance, registry *ruleset.FeatRegistry, roomNPCs []*npc.Instance) NPCEffectiveStats {
	return computeStats(attacker, target, registry, roomNPCs)
}

// computeStats is the shared implementation for ComputeNPCEffectiveStats and ComputeNPCAttackStats.
//
// Precondition: inst must not be nil; registry must not be nil.
// Postcondition: Returns an NPCEffectiveStats with summed bonuses from applicable passive feats.
func computeStats(inst *npc.Instance, target *npc.Instance, registry *ruleset.FeatRegistry, roomNPCs []*npc.Instance) NPCEffectiveStats {
	var stats NPCEffectiveStats
	for _, featID := range inst.Feats {
		f, ok := registry.Feat(featID)
		if !ok || !f.AllowNPC {
			continue
		}
		// Skip feats whose TargetTags do not match the target (REQ-AE-11).
		if len(f.TargetTags) > 0 {
			if target == nil {
				continue
			}
			matched := false
			for _, tag := range f.TargetTags {
				if target.HasTag(tag) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
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
