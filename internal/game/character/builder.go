package character

import (
	"errors"
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

// applyModifiers starts all abilities at 10 and adds region modifier values.
func applyModifiers(mods map[string]int) AbilityScores {
	a := AbilityScores{
		Brutality: 10, Grit: 10, Quickness: 10,
		Reasoning: 10, Savvy: 10, Flair: 10,
	}
	for ability, delta := range mods {
		switch ability {
		case "brutality":
			a.Brutality += delta
		case "grit":
			a.Grit += delta
		case "quickness":
			a.Quickness += delta
		case "reasoning":
			a.Reasoning += delta
		case "savvy":
			a.Savvy += delta
		case "flair":
			a.Flair += delta
		}
	}
	return a
}

// applyKeyAbilityBoost adds +2 to the class key ability score.
func applyKeyAbilityBoost(a AbilityScores, keyAbility string) AbilityScores {
	switch keyAbility {
	case "brutality":
		a.Brutality += 2
	case "grit":
		a.Grit += 2
	case "quickness":
		a.Quickness += 2
	case "reasoning":
		a.Reasoning += 2
	case "savvy":
		a.Savvy += 2
	case "flair":
		a.Flair += 2
	}
	return a
}

// BuildWithJob constructs a new Character from a name, region, job, and team.
// Ability scores start at 10, region modifiers are applied, then the
// job key ability receives a +2 boost. HP = max(1, hpPerLevel + GRT modifier).
//
// Precondition: name must be non-empty; region, job, and team must be non-nil.
// Postcondition: Returns a Character ready for persistence, or a non-nil error.
func BuildWithJob(name string, region *ruleset.Region, job *ruleset.Job, team *ruleset.Team) (*Character, error) {
	if name == "" {
		return nil, errors.New("character name must not be empty")
	}
	if region == nil {
		return nil, errors.New("region must not be nil")
	}
	if job == nil {
		return nil, errors.New("job must not be nil")
	}
	if team == nil {
		return nil, errors.New("team must not be nil")
	}

	abilities := applyModifiers(region.Modifiers)
	abilities = applyKeyAbilityBoost(abilities, job.KeyAbility)

	grtMod := abilities.Modifier(abilities.Grit)
	maxHP := job.HitPointsPerLevel + grtMod
	if maxHP < 1 {
		maxHP = 1
	}

	loc := team.StartRoom
	if loc == "" {
		loc = "grinders_row" // legacy fallback
	}

	return &Character{
		Name:      name,
		Region:    region.ID,
		Class:     job.ID,
		Team:      team.ID,
		Level:     1,
		Location:  loc,
		Abilities: abilities,
		MaxHP:     maxHP,
		CurrentHP: maxHP,
	}, nil
}

// BuildSkillsFromJob constructs a full skill proficiency map for a new character.
// Fixed skills and player-chosen skills are set to "trained". All others are "untrained".
//
// Precondition: allSkillIDs contains all valid skill IDs; chosen is a subset of job.SkillGrants.Choices.Pool.
// Postcondition: Returns a map with exactly len(allSkillIDs) entries.
func BuildSkillsFromJob(job *ruleset.Job, allSkillIDs []string, chosen []string) map[string]string {
	trained := make(map[string]bool)

	if job.SkillGrants != nil {
		for _, id := range job.SkillGrants.Fixed {
			trained[id] = true
		}
	}
	for _, id := range chosen {
		trained[id] = true
	}

	out := make(map[string]string, len(allSkillIDs))
	for _, id := range allSkillIDs {
		if trained[id] {
			out[id] = "trained"
		} else {
			out[id] = "untrained"
		}
	}
	return out
}

// BuildFeatsFromJob constructs the feat list for a new or backfilled character.
//
// Precondition: job must not be nil. chosen, generalChosen, skillChosen may be nil.
// Postcondition: Returns a slice containing all granted feat IDs (no duplicates).
func BuildFeatsFromJob(job *ruleset.Job, chosen []string, generalChosen []string, skillChosen []string) []string {
	seen := make(map[string]bool)
	var feats []string
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			feats = append(feats, id)
		}
	}
	if job.FeatGrants == nil {
		return feats
	}
	for _, id := range job.FeatGrants.Fixed {
		add(id)
	}
	for _, id := range chosen {
		add(id)
	}
	for _, id := range generalChosen {
		add(id)
	}
	for _, id := range skillChosen {
		add(id)
	}
	return feats
}

// BuildClassFeaturesFromJob returns all class feature IDs granted by the job.
// All grants are fixed; no player selection required.
//
// Precondition: job may be nil (returns nil in that case).
// Postcondition: Returns a slice of feature IDs (may be nil if job is nil or has no grants).
func BuildClassFeaturesFromJob(job *ruleset.Job) []string {
	if job == nil || len(job.ClassFeatureGrants) == 0 {
		return nil
	}
	result := make([]string, len(job.ClassFeatureGrants))
	copy(result, job.ClassFeatureGrants)
	return result
}

// ApplyAbilityBoosts returns the ability scores after stacking all boost sources.
// Order: archetype fixed → archetype free chosen → region fixed → region free chosen.
// Each boost adds +2. Nil grants and nil chosen slices are no-ops.
//
// Precondition: archetypeChosen and regionChosen satisfy within-source uniqueness (caller enforces).
// Postcondition: Each boosted ability increases by exactly +2 per application.
func ApplyAbilityBoosts(
	base AbilityScores,
	archetypeBoosts *ruleset.AbilityBoostGrant, archetypeChosen []string,
	regionBoosts *ruleset.AbilityBoostGrant, regionChosen []string,
) AbilityScores {
	result := base
	applyBoost := func(ability string) {
		switch ability {
		case "brutality":
			result.Brutality += 2
		case "grit":
			result.Grit += 2
		case "quickness":
			result.Quickness += 2
		case "reasoning":
			result.Reasoning += 2
		case "savvy":
			result.Savvy += 2
		case "flair":
			result.Flair += 2
		}
	}
	if archetypeBoosts != nil {
		for _, ab := range archetypeBoosts.Fixed {
			applyBoost(ab)
		}
	}
	for _, ab := range archetypeChosen {
		applyBoost(ab)
	}
	if regionBoosts != nil {
		for _, ab := range regionBoosts.Fixed {
			applyBoost(ab)
		}
	}
	for _, ab := range regionChosen {
		applyBoost(ab)
	}
	return result
}

// AbilityBoostPool returns the valid free-boost ability pool for a source.
// Excludes abilities in fixed and abilities already in alreadyChosen.
// Returns abilities in canonical order: brutality, grit, quickness, reasoning, savvy, flair.
//
// Precondition: fixed and alreadyChosen may be nil.
// Postcondition: No ability appears more than once in the returned slice.
func AbilityBoostPool(fixed []string, alreadyChosen []string) []string {
	excluded := make(map[string]bool)
	for _, ab := range fixed {
		excluded[ab] = true
	}
	for _, ab := range alreadyChosen {
		excluded[ab] = true
	}
	var pool []string
	for _, ab := range ruleset.AllAbilities() {
		if !excluded[ab] {
			pool = append(pool, ab)
		}
	}
	return pool
}

// skillRankOrder maps rank names to numeric values for comparison (REQ-JD-7).
var skillRankOrder = map[string]int{
	"untrained": 0,
	"trained":   1,
	"expert":    2,
	"master":    3,
	"legendary": 4,
}

// higherRank returns the higher of two skill rank strings.
func higherRank(a, b string) string {
	if skillRankOrder[b] > skillRankOrder[a] {
		return b
	}
	return a
}

// ComputeHeldJobBenefits aggregates skills and feats from all held jobs.
// Returns (skills map[skill_id]rank, feats []string) — deduped union.
// For overlapping skills, the highest rank wins (REQ-JD-7).
// feats has each feat ID exactly once (deduplicated).
// REQ-JD-14: Pure function with no side effects.
func ComputeHeldJobBenefits(jobs []*ruleset.Job) (map[string]string, []string) {
	skills := make(map[string]string)
	featSet := make(map[string]bool)
	for _, job := range jobs {
		if job.SkillGrants != nil {
			for _, s := range job.SkillGrants.Fixed {
				if existing, ok := skills[s]; ok {
					skills[s] = higherRank(existing, "trained")
				} else {
					skills[s] = "trained"
				}
			}
		}
		if job.FeatGrants != nil {
			for _, f := range job.FeatGrants.Fixed {
				featSet[f] = true
			}
		}
	}
	feats := make([]string, 0, len(featSet))
	for f := range featSet {
		feats = append(feats, f)
	}
	return skills, feats
}

// ComputeHeldJobBenefitsWithDrawbacks is like ComputeHeldJobBenefits but also
// returns passive stat modifiers from all held jobs' drawbacks.
// REQ-JD-9: Passive drawback stat modifiers included.
// REQ-JD-14: Pure function; no side effects.
func ComputeHeldJobBenefitsWithDrawbacks(jobs []*ruleset.Job) (map[string]string, []string, []ruleset.StatModifier) {
	skills, feats := ComputeHeldJobBenefits(jobs)
	var mods []ruleset.StatModifier
	seen := make(map[string]bool)
	for _, job := range jobs {
		for _, db := range job.Drawbacks {
			if db.Type == "passive" && db.StatModifier != nil {
				key := job.ID + ":" + db.ID
				if !seen[key] {
					seen[key] = true
					mods = append(mods, *db.StatModifier)
				}
			}
		}
	}
	return skills, feats, mods
}

// AbilityName returns the short display label for an ability score field.
func AbilityName(field string) string {
	names := map[string]string{
		"brutality": "BRT",
		"grit":      "GRT",
		"quickness": "QCK",
		"reasoning": "RSN",
		"savvy":     "SAV",
		"flair":     "FLR",
	}
	if n, ok := names[field]; ok {
		return n
	}
	return fmt.Sprintf("<%s>", field)
}
