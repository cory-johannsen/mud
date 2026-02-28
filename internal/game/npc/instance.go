package npc

import (
	"math/rand"
	"time"
)

// Instance is a live NPC entity occupying a room.
type Instance struct {
	// ID uniquely identifies this runtime instance.
	ID string
	// TemplateID is the source template's ID.
	TemplateID string
	// Name is copied from the template for display.
	Name string
	// Description is copied from the template.
	Description string
	// RoomID is the room this instance currently occupies.
	RoomID string
	// CurrentHP is the instance's current hit points.
	CurrentHP int
	// MaxHP is the instance's maximum hit points.
	MaxHP int
	// AC is the instance's armor class.
	AC int
	// Level is the instance's level.
	Level int
	// Perception is the instance's perception modifier.
	Perception int
	// AIDomain is the HTN domain ID copied from the template at spawn time.
	AIDomain string
	// Loot is the loot table copied from the template; nil means no loot.
	Loot *LootTable
	// Taunts is the list of taunt strings copied from the template.
	Taunts []string
	// TauntChance is the probability (0–1) of taunting on each check.
	TauntChance float64
	// TauntCooldown is the minimum duration between taunts.
	TauntCooldown time.Duration
	// LastTauntTime is the last time this NPC taunted.
	LastTauntTime time.Time
}

// NewInstance creates a live NPC instance from a template, placed in roomID.
//
// Precondition: id must be non-empty; tmpl must be non-nil; roomID must be non-empty.
// Postcondition: CurrentHP equals tmpl.MaxHP.
func NewInstance(id string, tmpl *Template, roomID string) *Instance {
	var cooldown time.Duration
	if tmpl.TauntCooldown != "" {
		cooldown, _ = time.ParseDuration(tmpl.TauntCooldown)
	}
	return &Instance{
		ID:            id,
		TemplateID:    tmpl.ID,
		Name:          tmpl.Name,
		Description:   tmpl.Description,
		RoomID:        roomID,
		CurrentHP:     tmpl.MaxHP,
		MaxHP:         tmpl.MaxHP,
		AC:            tmpl.AC,
		Level:         tmpl.Level,
		Perception:    tmpl.Perception,
		AIDomain:      tmpl.AIDomain,
		Loot:          tmpl.Loot,
		Taunts:        tmpl.Taunts,
		TauntChance:   tmpl.TauntChance,
		TauntCooldown: cooldown,
	}
}

// TryTaunt attempts to produce a taunt string, respecting chance and cooldown.
//
// Precondition: now must not be zero.
// Postcondition: Returns (taunt, true) if a taunt fires, updating LastTauntTime;
// returns ("", false) otherwise.
func (i *Instance) TryTaunt(now time.Time) (string, bool) {
	if len(i.Taunts) == 0 || i.TauntChance <= 0 {
		return "", false
	}
	if !i.LastTauntTime.IsZero() && now.Sub(i.LastTauntTime) < i.TauntCooldown {
		return "", false
	}
	if rand.Float64() >= i.TauntChance {
		return "", false
	}
	taunt := i.Taunts[rand.Intn(len(i.Taunts))]
	i.LastTauntTime = now
	return taunt, true
}

// IsDead reports whether the instance has zero or fewer hit points.
func (i *Instance) IsDead() bool {
	return i.CurrentHP <= 0
}

// HealthDescription returns a visible health state string suitable for examine output.
//
// Postcondition: Returns a non-empty string.
func (i *Instance) HealthDescription() string {
	if i.CurrentHP <= 0 {
		return "dead"
	}
	pct := float64(i.CurrentHP) / float64(i.MaxHP)
	switch {
	case pct >= 1.0:
		return "unharmed"
	case pct >= 0.85:
		return "barely scratched"
	case pct >= 0.60:
		return "lightly wounded"
	case pct >= 0.40:
		return "moderately wounded"
	case pct >= 0.20:
		return "heavily wounded"
	default:
		return "critically wounded"
	}
}
