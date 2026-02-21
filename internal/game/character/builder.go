package character

import (
	"errors"
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

// Build constructs a new Character from a name, region, and class.
// Ability scores start at 10, region modifiers are applied, then the
// class key ability receives a +2 boost. HP = max(1, hpPerLevel + CON modifier).
//
// Precondition: name must be non-empty; region and class must be non-nil.
// Postcondition: Returns a Character ready for persistence, or a non-nil error.
func Build(name string, region *ruleset.Region, class *ruleset.Class) (*Character, error) {
	if name == "" {
		return nil, errors.New("character name must not be empty")
	}
	if region == nil {
		return nil, errors.New("region must not be nil")
	}
	if class == nil {
		return nil, errors.New("class must not be nil")
	}

	abilities := applyModifiers(region.Modifiers)
	abilities = applyKeyAbilityBoost(abilities, class.KeyAbility)

	conMod := abilities.Modifier(abilities.Constitution)
	maxHP := class.HitPointsPerLevel + conMod
	if maxHP < 1 {
		maxHP = 1
	}

	return &Character{
		Name:      name,
		Region:    region.ID,
		Class:     class.ID,
		Level:     1,
		Location:  "grinders_row",
		Abilities: abilities,
		MaxHP:     maxHP,
		CurrentHP: maxHP,
	}, nil
}

// applyModifiers starts all abilities at 10 and adds region modifier values.
func applyModifiers(mods map[string]int) AbilityScores {
	a := AbilityScores{
		Strength: 10, Dexterity: 10, Constitution: 10,
		Intelligence: 10, Wisdom: 10, Charisma: 10,
	}
	for ability, delta := range mods {
		switch ability {
		case "strength":
			a.Strength += delta
		case "dexterity":
			a.Dexterity += delta
		case "constitution":
			a.Constitution += delta
		case "intelligence":
			a.Intelligence += delta
		case "wisdom":
			a.Wisdom += delta
		case "charisma":
			a.Charisma += delta
		}
	}
	return a
}

// applyKeyAbilityBoost adds +2 to the class key ability score.
func applyKeyAbilityBoost(a AbilityScores, keyAbility string) AbilityScores {
	switch keyAbility {
	case "strength":
		a.Strength += 2
	case "dexterity":
		a.Dexterity += 2
	case "constitution":
		a.Constitution += 2
	case "intelligence":
		a.Intelligence += 2
	case "wisdom":
		a.Wisdom += 2
	case "charisma":
		a.Charisma += 2
	}
	return a
}

// BuildWithJob constructs a new Character from a name, region, job, and team.
// Ability scores start at 10, region modifiers are applied, then the
// job key ability receives a +2 boost. HP = max(1, hpPerLevel + CON modifier).
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

	conMod := abilities.Modifier(abilities.Constitution)
	maxHP := job.HitPointsPerLevel + conMod
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
		"strength":     "STR",
		"dexterity":    "DEX",
		"constitution": "CON",
		"intelligence": "INT",
		"wisdom":       "WIS",
		"charisma":     "CHA",
	}
	if n, ok := names[field]; ok {
		return n
	}
	return fmt.Sprintf("<%s>", field)
}
