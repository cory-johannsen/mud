// internal/game/effect/resolve.go
package effect

import "sort"

// Resolved is the output of Resolve for one stat.
type Resolved struct {
	Stat         Stat
	Total        int
	Contributing []Contribution
	Suppressed   []Contribution
}

// Contribution is one bonus's contribution record.
type Contribution struct {
	EffectID     string
	SourceID     string
	CasterUID    string
	BonusType    BonusType
	Value        int
	OverriddenBy *ContributionRef // non-nil only in Suppressed list
}

// ContributionRef identifies the winning effect that caused suppression.
type ContributionRef struct {
	EffectID  string
	SourceID  string
	CasterUID string
}

// candidate is an internal working type for Resolve.
type candidate struct {
	effectID  string
	sourceID  string
	casterUID string
	bonusType BonusType
	value     int
}

// Resolve computes the net bonus for stat across all effects in set. (DEDUP-7: pure)
// A nil set returns a zero Resolved.
func Resolve(set *EffectSet, stat Stat) Resolved {
	if set == nil {
		return Resolved{Stat: stat}
	}

	// Collect all matching bonus contributions.
	var matches []candidate
	for _, e := range set.All() {
		for _, b := range e.Bonuses {
			if StatMatches(b.Stat, stat) {
				matches = append(matches, candidate{
					effectID:  e.EffectID,
					sourceID:  e.SourceID,
					casterUID: e.CasterUID,
					bonusType: b.Type,
					value:     b.Value,
				})
			}
		}
	}

	if len(matches) == 0 {
		return Resolved{Stat: stat}
	}

	var contributing []Contribution
	var suppressed []Contribution
	total := 0

	// Process typed buckets (status, circumstance, item) — highest bonus wins, worst penalty wins.
	for _, bt := range []BonusType{BonusTypeStatus, BonusTypeCircumstance, BonusTypeItem} {
		var pos, neg []candidate
		for _, m := range matches {
			if m.bonusType != bt {
				continue
			}
			if m.value > 0 {
				pos = append(pos, m)
			} else {
				neg = append(neg, m)
			}
		}

		if winner, losers := pickHighest(pos); winner != nil {
			total += winner.value
			contributing = append(contributing, toContribution(*winner, nil))
			ref := &ContributionRef{EffectID: winner.effectID, SourceID: winner.sourceID, CasterUID: winner.casterUID}
			for _, l := range losers {
				suppressed = append(suppressed, toContribution(l, ref))
			}
		}

		if winner, losers := pickLowest(neg); winner != nil {
			total += winner.value
			contributing = append(contributing, toContribution(*winner, nil))
			ref := &ContributionRef{EffectID: winner.effectID, SourceID: winner.sourceID, CasterUID: winner.casterUID}
			for _, l := range losers {
				suppressed = append(suppressed, toContribution(l, ref))
			}
		}
	}

	// Untyped: all stack.
	for _, m := range matches {
		if m.bonusType == BonusTypeUntyped {
			total += m.value
			contributing = append(contributing, toContribution(m, nil))
		}
	}

	return Resolved{Stat: stat, Total: total, Contributing: contributing, Suppressed: suppressed}
}

// pickHighest returns the candidate with the maximum value (lex tiebreak), plus the losers.
// Precondition: all candidates have value > 0.
func pickHighest(cs []candidate) (*candidate, []candidate) {
	if len(cs) == 0 {
		return nil, nil
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].value != cs[j].value {
			return cs[i].value > cs[j].value
		}
		// tie: ascending lex (SourceID, CasterUID) — first is winner (DEDUP-6)
		if cs[i].sourceID != cs[j].sourceID {
			return cs[i].sourceID < cs[j].sourceID
		}
		return cs[i].casterUID < cs[j].casterUID
	})
	return &cs[0], cs[1:]
}

// pickLowest returns the candidate with the minimum value (lex tiebreak), plus the losers.
// Precondition: all candidates have value < 0.
func pickLowest(cs []candidate) (*candidate, []candidate) {
	if len(cs) == 0 {
		return nil, nil
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].value != cs[j].value {
			return cs[i].value < cs[j].value
		}
		if cs[i].sourceID != cs[j].sourceID {
			return cs[i].sourceID < cs[j].sourceID
		}
		return cs[i].casterUID < cs[j].casterUID
	})
	return &cs[0], cs[1:]
}

func toContribution(c candidate, overriddenBy *ContributionRef) Contribution {
	return Contribution{
		EffectID:     c.effectID,
		SourceID:     c.sourceID,
		CasterUID:    c.casterUID,
		BonusType:    c.bonusType,
		Value:        c.value,
		OverriddenBy: overriddenBy,
	}
}
