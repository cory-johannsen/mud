// Package character defines the character domain model and pure creation logic.
package character

import "time"

// AbilityScores holds the six PF2E ability score values for a character.
type AbilityScores struct {
	Strength     int
	Dexterity    int
	Constitution int
	Intelligence int
	Wisdom       int
	Charisma     int
}

// Modifier returns the PF2E ability modifier for a given score: (score - 10) / 2.
func (a AbilityScores) Modifier(score int) int {
	return (score - 10) / 2
}

// Character represents a player character's persistent state.
//
// AccountID and ID are set by the persistence layer; zero values indicate an unsaved character.
type Character struct {
	ID        int64
	AccountID int64

	Name       string
	Region     string // home region ID
	Class      string // class ID
	Level      int
	Experience int

	Location  string // current room ID
	Abilities AbilityScores
	MaxHP     int
	CurrentHP int

	CreatedAt time.Time
	UpdatedAt time.Time
}
