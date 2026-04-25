package combat

import (
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/effect"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

// BuildEffectsOpts carries all sources needed to build a Combatant's EffectSet.
//
// Precondition: BearerUID must be non-empty.
// Any of Conditions / PassiveFeats / PassiveTechs may be nil (treated as empty).
type BuildEffectsOpts struct {
	// BearerUID is the combatant UID that owns the resulting EffectSet.
	BearerUID string
	// Conditions is the ActiveSet whose current conditions contribute bonuses.
	Conditions *condition.ActiveSet
	// PassiveFeats are the bearer's class features; only those with Active == false
	// and non-empty PassiveBonuses contribute.
	PassiveFeats []*ruleset.ClassFeature
	// PassiveTechs are the bearer's technologies; only those with Passive == true
	// and non-empty PassiveBonuses contribute.
	PassiveTechs []*technology.TechnologyDef
	// WeaponSourceID is the opaque identifier of the equipped weapon; empty means
	// no weapon-bonus effect will be produced.
	WeaponSourceID string
	// WeaponBonusValue is the item-type bonus on attack and damage granted by the
	// equipped weapon's "+" designation. Zero means no weapon-bonus effect will
	// be produced.
	WeaponBonusValue int
}

// BuildCombatantEffects constructs a fresh EffectSet populated from all effect
// sources declared in opts. Condition bonuses are scaled by stack count. Feat
// and tech entries that do not qualify (active feats, non-passive techs, empty
// PassiveBonuses) are skipped. A zero-value WeaponBonusValue or empty
// WeaponSourceID suppresses the weapon-bonus effect.
//
// Precondition: opts.BearerUID must be non-empty.
// Postcondition: returns a non-nil EffectSet.
func BuildCombatantEffects(opts BuildEffectsOpts) *effect.EffectSet {
	es := effect.NewEffectSet()

	// 1. Condition effects from ActiveSet — scale bonuses by stack count.
	if opts.Conditions != nil {
		for _, ac := range opts.Conditions.All() {
			if ac == nil || ac.Def == nil || len(ac.Def.Bonuses) == 0 {
				continue
			}
			bonuses := make([]effect.Bonus, len(ac.Def.Bonuses))
			for i, b := range ac.Def.Bonuses {
				scaled := b
				scaled.Value = b.Value * ac.Stacks
				bonuses[i] = scaled
			}
			es.Apply(effect.Effect{
				EffectID:  ac.Def.ID,
				SourceID:  "condition:" + ac.Def.ID,
				CasterUID: opts.BearerUID,
				Bonuses:   bonuses,
				DurKind:   effect.DurationUntilRemove,
			})
		}
	}

	// 2. Feat passive bonuses — only for non-active feats with declared bonuses.
	for _, f := range opts.PassiveFeats {
		if f == nil || f.Active || len(f.PassiveBonuses) == 0 {
			continue
		}
		es.Apply(effect.Effect{
			EffectID:  f.ID,
			SourceID:  "feat:" + f.ID,
			CasterUID: opts.BearerUID,
			Bonuses:   f.PassiveBonuses,
			DurKind:   effect.DurationUntilRemove,
		})
	}

	// 3. Tech passive bonuses — only for passive techs with declared bonuses.
	for _, td := range opts.PassiveTechs {
		if td == nil || !td.Passive || len(td.PassiveBonuses) == 0 {
			continue
		}
		es.Apply(effect.Effect{
			EffectID:  td.ID,
			SourceID:  "tech:" + td.ID,
			CasterUID: opts.BearerUID,
			Bonuses:   td.PassiveBonuses,
			DurKind:   effect.DurationUntilRemove,
		})
	}

	// 4. Weapon item bonus — item-typed bonuses to attack and damage.
	if opts.WeaponSourceID != "" && opts.WeaponBonusValue != 0 {
		es.Apply(effect.Effect{
			EffectID:  opts.WeaponSourceID,
			SourceID:  "item:" + opts.WeaponSourceID,
			CasterUID: "",
			Bonuses: []effect.Bonus{
				{Stat: effect.StatAttack, Value: opts.WeaponBonusValue, Type: effect.BonusTypeItem},
				{Stat: effect.StatDamage, Value: opts.WeaponBonusValue, Type: effect.BonusTypeItem},
			},
			DurKind: effect.DurationUntilRemove,
		})
	}

	return es
}

// SyncConditionApply updates cbt.Effects when a condition is applied or its
// stack count changes mid-combat. Stacks scale the condition's bonus values.
//
// Precondition: cbt must be non-nil; def must be non-nil; stacks >= 1.
// Postcondition: cbt.Effects is initialised if nil and contains an effect
// keyed by SourceID="condition:"+def.ID for the given caster UID.
func SyncConditionApply(cbt *Combatant, uid string, def *condition.ConditionDef, stacks int) {
	if cbt == nil || def == nil {
		return
	}
	if cbt.Effects == nil {
		cbt.Effects = effect.NewEffectSet()
	}
	if len(def.Bonuses) == 0 {
		return
	}
	bonuses := make([]effect.Bonus, len(def.Bonuses))
	for i, b := range def.Bonuses {
		scaled := b
		scaled.Value = b.Value * stacks
		bonuses[i] = scaled
	}
	cbt.Effects.Apply(effect.Effect{
		EffectID:  def.ID,
		SourceID:  "condition:" + def.ID,
		CasterUID: uid,
		Bonuses:   bonuses,
		DurKind:   effect.DurationUntilRemove,
	})
}

// SyncConditionRemove updates cbt.Effects when a condition is removed mid-combat.
//
// Precondition: conditionID must be non-empty.
// Postcondition: no effect with SourceID="condition:"+conditionID remains in cbt.Effects.
func SyncConditionRemove(cbt *Combatant, conditionID string) {
	if cbt == nil || cbt.Effects == nil {
		return
	}
	cbt.Effects.RemoveBySource("condition:" + conditionID)
}

// SyncConditionsTick updates cbt.Effects after an ActiveSet.Tick expires the
// given condition IDs. Each expired condition's associated effect is removed.
//
// Precondition: cbt may be nil (no-op); expiredIDs may be nil or empty.
// Postcondition: no effect with SourceID="condition:"+id remains for any id in expiredIDs.
func SyncConditionsTick(cbt *Combatant, expiredIDs []string) {
	if cbt == nil || cbt.Effects == nil {
		return
	}
	for _, id := range expiredIDs {
		cbt.Effects.RemoveBySource("condition:" + id)
	}
}

// OverrideNarrativeEvents computes which effects stopped contributing to each
// stat in stats between the before and after EffectSets due to suppression by
// a higher-priority same-type bonus. Returns one "[EFFECT] ... is overridden
// by ..." line per newly-suppressed contribution.
//
// Precondition: stats may be empty (returns empty events).
// Postcondition: events are appended in order (stats, then suppressed entries).
func OverrideNarrativeEvents(before, after *effect.EffectSet, stats []effect.Stat) []string {
	var events []string
	for _, stat := range stats {
		rb := effect.Resolve(before, stat)
		ra := effect.Resolve(after, stat)
		for _, sup := range ra.Suppressed {
			wasContributing := false
			for _, c := range rb.Contributing {
				if c.EffectID == sup.EffectID {
					wasContributing = true
					break
				}
			}
			if !wasContributing {
				continue
			}
			var winnerSource string
			if sup.OverriddenBy != nil {
				winnerSource = sup.OverriddenBy.SourceID
			}
			events = append(events, fmt.Sprintf("[EFFECT] %s (%s) is overridden by %s.",
				sup.SourceID, stat, winnerSource))
		}
	}
	return events
}
