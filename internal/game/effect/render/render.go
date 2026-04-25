// Package render formats EffectSet data for human consumption. It is the
// canonical formatter shared by the telnet renderer, the gameserver
// CharacterSheetView projection, and any future UI surface that presents
// active effects.
//
// This package deliberately has no dependency on frontend/handlers, proto
// code, or other UI layers — it consumes only internal/game/effect. That lets
// both the Go gameserver (which populates CharacterSheetView.EffectsSummary)
// and the telnet frontend reuse the same output.
package render

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/effect"
)

// effectsAnnotation is one rendered bonus-line annotation under an effect.
// suppressed == true marks that the bonus was overridden by a winning
// contribution elsewhere in the set.
type effectsAnnotation struct {
	line       string
	suppressed bool
}

// statList is the canonical ordered list of Stat values that EffectsBlock
// probes when classifying per-effect contributions.
//
// Precondition: every member must be a Stat constant exported by the effect
// package.
// Postcondition: the slice is read-only.
var statList = []effect.Stat{
	effect.StatAttack,
	effect.StatAC,
	effect.StatDamage,
	effect.StatSpeed,
	effect.StatBrutality,
	effect.StatGrit,
	effect.StatQuickness,
	effect.StatReasoning,
	effect.StatSavvy,
	effect.StatFlair,
	effect.StatSkill,
}

// EffectsBlock formats an EffectSet as a human-readable multi-line block.
//
// Precondition: es may be nil or empty. casterNames may be nil. width must be
// non-negative (currently reserved for future word-wrap and is not otherwise
// consulted).
// Postcondition: returns a multi-line string beginning with "Effects:". For
// each active effect, a row is emitted that identifies the source, caster,
// and either every stat contribution with active/(overridden) status or the
// sentinel "[no stat bonuses]" when the effect declares no Bonuses.
func EffectsBlock(es *effect.EffectSet, casterNames map[string]string, width int) string {
	_ = width // reserved for future word-wrap
	if es == nil || len(es.All()) == 0 {
		return "Effects:\n  No active effects.\n"
	}

	annotMap := map[string][]effectsAnnotation{}
	for _, stat := range statList {
		r := effect.Resolve(es, stat)
		for _, c := range r.Contributing {
			sign := "+"
			if c.Value < 0 {
				sign = ""
			}
			line := fmt.Sprintf("%s %s%d %s", stat, sign, c.Value, strings.ToLower(string(c.BonusType)))
			annotMap[contributionKey(c.EffectID, c.SourceID, c.CasterUID)] = append(
				annotMap[contributionKey(c.EffectID, c.SourceID, c.CasterUID)],
				effectsAnnotation{line: line, suppressed: false},
			)
		}
		for _, c := range r.Suppressed {
			sign := "+"
			if c.Value < 0 {
				sign = ""
			}
			overriddenBy := "(unknown)"
			if c.OverriddenBy != nil {
				overriddenBy = c.OverriddenBy.SourceID
			}
			line := fmt.Sprintf("%s %s%d %s  (overridden by %s)",
				stat, sign, c.Value, strings.ToLower(string(c.BonusType)), overriddenBy)
			annotMap[contributionKey(c.EffectID, c.SourceID, c.CasterUID)] = append(
				annotMap[contributionKey(c.EffectID, c.SourceID, c.CasterUID)],
				effectsAnnotation{line: line, suppressed: true},
			)
		}
	}

	var sb strings.Builder
	sb.WriteString("Effects:\n")
	for _, e := range es.All() {
		casterLabel := "self"
		switch {
		case strings.HasPrefix(e.SourceID, "item:"):
			casterLabel = "item"
		case strings.HasPrefix(e.SourceID, "feat:"):
			casterLabel = "feat"
		case strings.HasPrefix(e.SourceID, "tech:"):
			casterLabel = "tech"
		}
		if e.CasterUID != "" {
			if name, ok := casterNames[e.CasterUID]; ok {
				casterLabel = "from " + name
			}
		}
		displayName := e.Annotation
		if displayName == "" {
			displayName = e.SourceID
		}
		annots := annotMap[contributionKey(e.EffectID, e.SourceID, e.CasterUID)]
		if len(annots) == 0 {
			fmt.Fprintf(&sb, "  %-25s (%-12s)  [no stat bonuses]\n", displayName, casterLabel)
			continue
		}
		first := true
		for _, a := range annots {
			status := "(active)"
			if a.suppressed {
				status = "(overridden)"
			}
			if first {
				fmt.Fprintf(&sb, "  %-25s (%-12s)  %-35s %s\n", displayName, casterLabel, a.line, status)
				first = false
			} else {
				fmt.Fprintf(&sb, "  %-25s  %-12s   %-35s %s\n", "", "", a.line, status)
			}
		}
	}
	return sb.String()
}

// contributionKey builds a stable identity for pairing a contribution to its
// originating Effect inside annotMap. It composes effectID, sourceID, and
// casterUID so that two effects sharing an EffectID but differing SourceID
// or CasterUID do not collide.
func contributionKey(effectID, sourceID, casterUID string) string {
	return effectID + "\x1f" + sourceID + "\x1f" + casterUID
}
