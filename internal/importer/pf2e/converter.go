package pf2e

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/cory-johannsen/mud/internal/importer"
)

var traditionMap = map[string]technology.Tradition{
	"occult": technology.TraditionNeural,
	"primal": technology.TraditionBioSynthetic,
	"arcane": technology.TraditionTechnical,
	"divine": technology.TraditionFanaticDoctrine,
}

var feetRegexp = regexp.MustCompile(`^\d+\s*feet?$`)
var roundsRegexp = regexp.MustCompile(`^(\d+)\s*rounds?$`)
var diceRegexp = regexp.MustCompile(`^(\d+)d(\d+)$`)
var htmlTagRegexp = regexp.MustCompile(`<[^>]+>`)
var uuidRegexp = regexp.MustCompile(`@UUID\[[^\]]+\]\{([^}]+)\}`)

var recognizedConditions = map[string]bool{
	"slowed": true, "immobilized": true, "blinded": true,
	"fleeing": true, "frightened": true, "stunned": true, "flat-footed": true,
}

// ConvertSpell transforms a parsed PF2ESpell into one TechData per matching
// Gunchete tradition.
//
// Precondition: spell must be non-nil.
// Postcondition: returns one TechData per matching tradition (may be empty),
// a slice of warning strings, and nil error on success; non-nil error on fatal failure.
// Spells with no matching tradition are skipped with a warning, not an error.
func ConvertSpell(spell *PF2ESpell) ([]*importer.TechData, []string, error) {
	var warnings []string

	var matchedTraditions []technology.Tradition
	for _, t := range spell.System.Traits.Traditions {
		if trad, ok := traditionMap[strings.ToLower(t)]; ok {
			matchedTraditions = append(matchedTraditions, trad)
		}
	}
	if len(matchedTraditions) == 0 {
		warnings = append(warnings, fmt.Sprintf(
			"spell %q has no matching tradition (traditions: %v); skipping",
			spell.Name, spell.System.Traits.Traditions,
		))
		return nil, warnings, nil
	}

	baseID := importer.NameToID(spell.Name)
	multi := len(matchedTraditions) > 1

	actionCost, acWarn := parseActionCost(spell.System.Time.Value, spell.Name)
	if acWarn != "" {
		warnings = append(warnings, acWarn)
	}

	rng, rngWarn := parseRange(spell.System.Range.Value, spell.System.Area)
	if rngWarn != "" {
		warnings = append(warnings, rngWarn)
	}

	targets := parseTargets(rng, spell.System.Target.Value)

	duration, durWarn := parseDuration(spell.System.Duration.Value, spell.Name)
	if durWarn != "" {
		warnings = append(warnings, durWarn)
	}

	resolution, saveType, saveDC := parseResolution(spell.System.Description.Value, spell.System.Traits.Value)

	effects := buildEffects(spell, resolution, duration)

	var results []*importer.TechData
	for _, trad := range matchedTraditions {
		id := baseID
		if multi {
			id = baseID + "_" + string(trad)
		}
		def := &technology.TechnologyDef{
			ID:          id,
			Name:        spell.Name,
			Description: stripHTML(spell.System.Description.Value),
			Tradition:   trad,
			Level:       spell.System.Level.Value,
			UsageType:   technology.UsagePrepared,
			ActionCost:  actionCost,
			Range:       rng,
			Targets:     targets,
			Duration:    duration,
			Resolution:  resolution,
			SaveType:    saveType,
			SaveDC:      saveDC,
			Effects:     effects,
		}
		results = append(results, &importer.TechData{
			Def:       def,
			Tradition: string(trad),
		})
	}
	return results, warnings, nil
}

func stripHTML(s string) string {
	// Replace UUID links with their display text
	s = uuidRegexp.ReplaceAllString(s, "$1")
	// Strip remaining HTML tags
	s = htmlTagRegexp.ReplaceAllString(s, "")
	// Collapse whitespace
	s = strings.TrimSpace(s)
	return s
}

func parseActionCost(val, spellName string) (int, string) {
	switch val {
	case "1":
		return 1, ""
	case "2":
		return 2, ""
	case "3":
		return 3, ""
	case "reaction", "free":
		return 0, ""
	default:
		return 2, fmt.Sprintf("spell %q: unrecognized action cost %q; defaulting to 2", spellName, val)
	}
}

func parseRange(rangeVal string, area *SpellArea) (technology.Range, string) {
	if area != nil {
		at := strings.ToLower(area.Type)
		if at == "emanation" || at == "burst" || at == "cone" {
			return technology.RangeZone, ""
		}
	}

	rv := strings.ToLower(strings.TrimSpace(rangeVal))

	if strings.Contains(rv, "emanation") || strings.Contains(rv, "burst") || strings.Contains(rv, "cone") {
		return technology.RangeZone, ""
	}
	switch rv {
	case "touch", "melee":
		return technology.RangeMelee, ""
	case "self", "":
		return technology.RangeSelf, ""
	}
	if feetRegexp.MatchString(rv) {
		return technology.RangeRanged, ""
	}
	return technology.RangeRanged, fmt.Sprintf("unrecognized range %q; defaulting to ranged", rangeVal)
}

func parseTargets(rng technology.Range, targetVal string) technology.Targets {
	if rng == technology.RangeZone {
		return technology.TargetsZone
	}
	tv := strings.ToLower(targetVal)
	if strings.Contains(tv, "all enemies") {
		return technology.TargetsAllEnemies
	}
	if strings.Contains(tv, "all allies") {
		return technology.TargetsAllAllies
	}
	return technology.TargetsSingle
}

func parseDuration(val, spellName string) (string, string) {
	v := strings.ToLower(strings.TrimSpace(val))
	switch v {
	case "instant", "instantaneous", "":
		return "instant", ""
	case "1 round", "sustained":
		return "rounds:1", ""
	case "1 minute":
		return "minutes:1", ""
	}
	if m := roundsRegexp.FindStringSubmatch(v); m != nil {
		return "rounds:" + m[1], ""
	}
	return "instant", fmt.Sprintf("spell %q: unrecognized duration %q; defaulting to instant", spellName, val)
}

func parseResolution(description string, traits []string) (resolution, saveType string, saveDC int) {
	saveType, found := parseSaveFromDescription(description)
	if found {
		return "save", saveType, 15
	}
	for _, t := range traits {
		if strings.ToLower(t) == "attack" {
			return "attack", "", 0
		}
	}
	return "none", "", 0
}

func parseSaveFromDescription(desc string) (saveType string, found bool) {
	lower := strings.ToLower(desc)
	if strings.Contains(lower, "will save") {
		return "cool", true
	}
	if strings.Contains(lower, "fortitude save") {
		return "toughness", true
	}
	if strings.Contains(lower, "reflex save") {
		return "hustle", true
	}
	return "", false
}

func buildEffects(spell *PF2ESpell, resolution, duration string) technology.TieredEffects {
	var effects technology.TieredEffects

	damageEffects := buildDamageEffects(spell.System.Damage)
	conditionEffects := buildConditionEffects(spell.System.Traits.Value, duration)
	allDirect := append(damageEffects, conditionEffects...)

	if len(allDirect) == 0 {
		desc := stripHTML(spell.System.Description.Value)
		if len(desc) > 200 {
			desc = strings.TrimSpace(desc[:200])
		}
		effects.OnApply = []technology.TechEffect{{
			Type:        technology.EffectUtility,
			Description: desc,
		}}
		return effects
	}

	switch resolution {
	case "save":
		effects.OnSuccess = halfStepEffects(damageEffects, conditionEffects)
		effects.OnFailure = allDirect
		effects.OnCritFailure = doubleEffects(damageEffects, conditionEffects)
	case "attack":
		effects.OnHit = allDirect
		effects.OnCritHit = doubleEffects(damageEffects, conditionEffects)
	default:
		effects.OnApply = allDirect
	}
	return effects
}

func buildDamageEffects(damage map[string]SpellDamageEntry) []technology.TechEffect {
	if len(damage) == 0 {
		return nil
	}
	keys := make([]string, 0, len(damage))
	for k := range damage {
		keys = append(keys, k)
	}
	sortStrings(keys)
	var effects []technology.TechEffect
	for _, k := range keys {
		entry := damage[k]
		if entry.Formula == "" {
			continue
		}
		effects = append(effects, technology.TechEffect{
			Type:       technology.EffectDamage,
			Dice:       entry.Formula,
			DamageType: entry.DamageType,
		})
	}
	return effects
}

func buildConditionEffects(traits []string, duration string) []technology.TechEffect {
	var effects []technology.TechEffect
	seen := map[string]bool{}
	for _, t := range traits {
		tl := strings.ToLower(t)
		if recognizedConditions[tl] && !seen[tl] {
			seen[tl] = true
			effects = append(effects, technology.TechEffect{
				Type:        technology.EffectCondition,
				ConditionID: tl,
				Value:       1,
				Duration:    duration,
			})
		}
	}
	return effects
}

func halfStepEffects(damageEffects, conditionEffects []technology.TechEffect) []technology.TechEffect {
	var out []technology.TechEffect
	for _, e := range damageEffects {
		half := e
		half.Dice = halfStepDice(e.Dice)
		out = append(out, half)
	}
	return append(out, conditionEffects...)
}

func doubleEffects(damageEffects, conditionEffects []technology.TechEffect) []technology.TechEffect {
	var out []technology.TechEffect
	for _, e := range damageEffects {
		d := e
		d.Dice = doubleDice(e.Dice)
		out = append(out, d)
	}
	return append(out, conditionEffects...)
}

func halfStepDice(dice string) string {
	count, sides, ok := parseDice(dice)
	if !ok {
		return dice
	}
	newSides := halfStep(sides)
	if newSides == 0 {
		return strconv.Itoa(count)
	}
	return fmt.Sprintf("%dd%d", count, newSides)
}

func doubleDice(dice string) string {
	count, sides, ok := parseDice(dice)
	if !ok {
		return dice
	}
	return fmt.Sprintf("%dd%d", count*2, sides)
}

func parseDice(dice string) (count, sides int, ok bool) {
	m := diceRegexp.FindStringSubmatch(strings.TrimSpace(dice))
	if m == nil {
		return 0, 0, false
	}
	c, err1 := strconv.Atoi(m[1])
	s, err2 := strconv.Atoi(m[2])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return c, s, true
}

func halfStep(sides int) int {
	switch sides {
	case 12:
		return 8
	case 10:
		return 6
	case 8:
		return 4
	case 6:
		return 3
	default:
		return 0
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
