// Package character defines the character domain model and pure creation logic.
package character

import "time"

// AbilityScores holds the six ability score values for a character.
type AbilityScores struct {
	Brutality int
	Grit      int
	Quickness int
	Reasoning int
	Savvy     int
	Flair     int
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
	Class      string // job ID (replaces class for Gunchete)
	Team       string // team ID: "gun" or "machete"
	Level      int
	Experience int

	Location  string // current room ID
	Abilities AbilityScores
	MaxHP     int
	CurrentHP int

	// DefaultCombatAction is the player's preferred combat action when no explicit choice is made.
	// Persisted to DB; "pass" when unset.
	DefaultCombatAction string

	// Gender is the player-selected gender identity string.
	// One of "male", "female", "non-binary", "indeterminate", or "custom:<text>".
	// Empty string means unset (will be backfilled at login).
	Gender string

	// Skills maps skill_id to proficiency rank for this character.
	// Populated after creation or loading from DB.
	Skills map[string]string

	// Feats is the list of feat IDs held by this character.
	// Populated after creation or loading from DB.
	Feats []string

	// ClassFeatures is the list of class feature IDs granted by the character's job.
	// Populated after creation or loading from DB.
	ClassFeatures []string

	CreatedAt time.Time
	UpdatedAt time.Time

	// DetainedUntil is non-nil when the character is serving a detention sentence.
	// Persisted to DB; nil means not detained.
	DetainedUntil *time.Time
}
