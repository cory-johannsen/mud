package npc

import (
	"math/rand"
	"sync"
	"time"

	"github.com/cory-johannsen/mud/internal/game/skillcheck"
)

// Instance is a live NPC entity occupying a room.
type Instance struct {
	// ID uniquely identifies this runtime instance.
	ID string
	// TemplateID is the source template's ID.
	TemplateID string
	// Type is the NPC category copied from the template, used for predators_eye matching.
	// Empty string means no category is defined.
	Type string
	// baseName is the unsuffixed name copied from the template at spawn time.
	baseName string
	// nameMu protects the name field.
	nameMu sync.RWMutex
	// name is the display name, potentially suffixed when multiple same-template
	// instances share a room.
	name string
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
	// Stealth is the instance's stealth modifier.
	Stealth int `yaml:"stealth"`
	// Deception is the instance's deception skill modifier.
	Deception int `yaml:"deception"`
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
	// SkillChecks defines skill check triggers fired when a player greets this NPC.
	SkillChecks []skillcheck.TriggerDef
	// Resistances maps damage type → flat reduction. Copied from template at spawn.
	Resistances map[string]int
	// Weaknesses maps damage type → flat bonus. Copied from template at spawn.
	Weaknesses map[string]int
	// WeaponID is the weapon item ID selected at spawn. Empty = unarmed.
	WeaponID string
	// ArmorID is the armor item ID selected at spawn. Empty = no armor.
	ArmorID string
	// UseCover is copied from the template's Combat.UseCover at spawn time.
	// When true, the NPC automatically takes cover at the start of its turn.
	UseCover bool
	// Brutality is copied from the template's Abilities.Brutality at spawn.
	// Used to compute Toughness DC.
	Brutality int
	// Quickness is copied from the template's Abilities.Quickness at spawn.
	// Used to compute Hustle DC.
	Quickness int
	// Savvy is copied from the template's Abilities.Savvy at spawn.
	// Used to compute Cool DC.
	Savvy int
	// ToughnessRank is the Toughness save proficiency rank, copied from template.
	ToughnessRank string
	// HustleRank is the Hustle save proficiency rank, copied from template.
	HustleRank string
	// CoolRank is the Cool save proficiency rank, copied from template.
	CoolRank string
}

// Name returns the instance's current display name.
func (i *Instance) Name() string {
	i.nameMu.RLock()
	defer i.nameMu.RUnlock()
	return i.name
}

// setName sets the instance's display name.
func (i *Instance) setName(s string) {
	i.nameMu.Lock()
	defer i.nameMu.Unlock()
	i.name = s
}

// pickWeighted selects one EquipmentEntry ID using weighted random selection.
// Returns "" if entries is empty or all weights are zero.
//
// Precondition: entries must not be nil.
func pickWeighted(entries []EquipmentEntry) string {
	total := 0
	for _, e := range entries {
		total += e.Weight
	}
	if total <= 0 {
		return ""
	}
	roll := rand.Intn(total)
	for _, e := range entries {
		roll -= e.Weight
		if roll < 0 {
			return e.ID
		}
	}
	return entries[len(entries)-1].ID
}

// NewInstanceWithResolver creates a live NPC instance from a template, placed in roomID.
// armorACBonus is an optional func(armorID string) int that returns the armor's AC bonus;
// pass nil to skip AC adjustment.
//
// Precondition: id must be non-empty; tmpl must be non-nil; roomID must be non-empty.
// Postcondition: CurrentHP equals tmpl.MaxHP; WeaponID and ArmorID are set from weighted roll.
func NewInstanceWithResolver(id string, tmpl *Template, roomID string, armorACBonus func(string) int) *Instance {
	var cooldown time.Duration
	if tmpl.TauntCooldown != "" {
		cooldown, _ = time.ParseDuration(tmpl.TauntCooldown)
	}

	weaponID := pickWeighted(tmpl.Weapon)
	armorID := pickWeighted(tmpl.Armor)
	ac := tmpl.AC
	if armorID != "" && armorACBonus != nil {
		ac += armorACBonus(armorID)
	}

	return &Instance{
		ID:            id,
		TemplateID:    tmpl.ID,
		Type:          tmpl.Type,
		name:          tmpl.Name,
		baseName:      tmpl.Name,
		Description:   tmpl.Description,
		RoomID:        roomID,
		CurrentHP:     tmpl.MaxHP,
		MaxHP:         tmpl.MaxHP,
		AC:            ac,
		Level:         tmpl.Level,
		Perception:    tmpl.Perception,
		Deception:     tmpl.Deception,
		AIDomain:      tmpl.AIDomain,
		Loot:          tmpl.Loot,
		Taunts:        tmpl.Taunts,
		TauntChance:   tmpl.TauntChance,
		TauntCooldown: cooldown,
		SkillChecks:   tmpl.SkillChecks,
		Resistances:   tmpl.Resistances,
		Weaknesses:    tmpl.Weaknesses,
		WeaponID:      weaponID,
		ArmorID:       armorID,
		UseCover:      tmpl.Combat.UseCover,
		Brutality:     tmpl.Abilities.Brutality,
		Quickness:     tmpl.Abilities.Quickness,
		Savvy:         tmpl.Abilities.Savvy,
		ToughnessRank: tmpl.ToughnessRank,
		HustleRank:    tmpl.HustleRank,
		CoolRank:      tmpl.CoolRank,
	}
}

// NewInstance creates a live NPC instance from a template with no armor AC resolver.
// Use NewInstanceWithResolver when an inventory registry is available to apply AC bonuses.
//
// Precondition: id must be non-empty; tmpl must be non-nil; roomID must be non-empty.
// Postcondition: CurrentHP equals tmpl.MaxHP; WeaponID/ArmorID are set; AC is base only.
func NewInstance(id string, tmpl *Template, roomID string) *Instance {
	return NewInstanceWithResolver(id, tmpl, roomID, nil)
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
