package combat

import "github.com/cory-johannsen/mud/internal/game/effect"

// CoverTier represents the tier of cover a combatant is behind.
// Tiers are ordered: NoCover < Lesser < Standard < Greater.
type CoverTier int

const (
	NoCover CoverTier = iota
	Lesser
	Standard
	Greater
)

func (t CoverTier) String() string {
	switch t {
	case Lesser:
		return "lesser"
	case Standard:
		return "standard"
	case Greater:
		return "greater"
	default:
		return "none"
	}
}

// CoverTierFromString converts a canonical tier name to a CoverTier.
// Unknown strings (including empty) map to NoCover.
func CoverTierFromString(s string) CoverTier {
	switch s {
	case "lesser":
		return Lesser
	case "standard":
		return Standard
	case "greater":
		return Greater
	default:
		return NoCover
	}
}

// DetermineCoverTier returns the cover tier the target is behind.
// In the 1D linear combat model, cover is carried on the combatant via
// Combatant.CoverTier. Returns NoCover when the target is not in cover.
func DetermineCoverTier(target *Combatant) CoverTier {
	if target == nil {
		return NoCover
	}
	return CoverTierFromString(target.CoverTier)
}

// AC and Quickness bonus magnitudes for each cover tier.
const (
	CoverACBonusLesser   = 1
	CoverACBonusStandard = 2
	CoverACBonusGreater  = 4
	CoverQKBonusStandard = 2
	CoverQKBonusGreater  = 4
)

// CoverBonus returns the AC and Quickness circumstance bonus magnitudes for the given tier.
func CoverBonus(t CoverTier) (acBonus, quicknessBonus int) {
	switch t {
	case Lesser:
		return CoverACBonusLesser, 0
	case Standard:
		return CoverACBonusStandard, CoverQKBonusStandard
	case Greater:
		return CoverACBonusGreater, CoverQKBonusGreater
	default:
		return 0, 0
	}
}

// CoverSourceID returns the canonical SourceID for the cover effect at the given tier.
func CoverSourceID(t CoverTier) string {
	return "cover:" + t.String()
}

// BuildCoverEffect constructs an ephemeral circumstance-typed Effect that contributes
// the AC (and Quickness when applicable) bonus for the given tier. Returns an Effect
// with no Bonuses when t == NoCover.
func BuildCoverEffect(t CoverTier) effect.Effect {
	ac, qk := CoverBonus(t)
	bonuses := make([]effect.Bonus, 0, 2)
	if ac != 0 {
		bonuses = append(bonuses, effect.Bonus{
			Stat:  effect.StatAC,
			Value: ac,
			Type:  effect.BonusTypeCircumstance,
		})
	}
	if qk != 0 {
		bonuses = append(bonuses, effect.Bonus{
			Stat:  effect.StatQuickness,
			Value: qk,
			Type:  effect.BonusTypeCircumstance,
		})
	}
	return effect.Effect{
		EffectID:   "cover_" + t.String(),
		SourceID:   CoverSourceID(t),
		CasterUID:  "",
		Bonuses:    bonuses,
		DurKind:    effect.DurationUntilRemove,
		Annotation: "cover (" + t.String() + ")",
	}
}

// WithCoverEffect applies the cover bonus effect to target.Effects for the duration
// of fn, then removes it.
func WithCoverEffect(target *Combatant, tier CoverTier, fn func()) {
	if tier > NoCover && target != nil && target.Effects != nil {
		target.Effects.Apply(BuildCoverEffect(tier))
		defer target.Effects.Remove(CoverSourceID(tier), "")
	}
	fn()
}
