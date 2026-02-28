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

	return &Character{
		Name:      name,
		Region:    region.ID,
		Class:     job.ID,
		Team:      team.ID,
		Level:     1,
		Location:  "grinders_row",
		Abilities: abilities,
		MaxHP:     maxHP,
		CurrentHP: maxHP,
	}, nil
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
