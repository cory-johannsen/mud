package npc

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/skillcheck"
	"github.com/cory-johannsen/mud/internal/game/xp"
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
	// Gender is propagated from Template.Gender at spawn (REQ-ZN-6).
	// Runtime-only: no YAML tag. Per-instance override not supported.
	Gender string
	// SeductionRejected maps player UID → true when this NPC has rejected a seduction
	// attempt from that player (REQ-ZN-8). Runtime-only: no YAML tag. Nil until first rejection.
	SeductionRejected map[string]bool
	// SeductionProbability is propagated from Template at spawn.
	SeductionProbability float64
	// SeductionGender is propagated from Template at spawn.
	SeductionGender string
	// Flair is the NPC's Flair ability score, propagated from Template.Abilities.Flair at spawn.
	Flair int
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
	// Awareness is the instance's awareness modifier.
	Awareness int
	// Stealth is the instance's stealth modifier.
	Stealth int `yaml:"stealth"`
	// Hustle is the instance's hustle skill modifier.
	Hustle int `yaml:"hustle"`
	// AIDomain is the HTN domain ID copied from the template at spawn time.
	AIDomain string
	// Loot is the loot table copied from the template; nil means no loot.
	Loot *LootTable
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
	// RobPercent is the fraction of a defeated player's currency this NPC steals,
	// as a percentage in [5.0, 30.0]. 0 means this NPC never robs.
	// Computed once at spawn from template RobMultiplier, level, and randomness.
	RobPercent float64
	// Currency is the NPC's wallet accumulated from robbing players.
	// Added to loot payout when the NPC dies. Zero at spawn.
	Currency int
	// SenseAbilities lists named special abilities copied from the template at spawn.
	SenseAbilities []string
	// Tags is the list of content labels propagated from the template at spawn.
	Tags []string
	// Feats is the list of feat IDs propagated from the template at spawn.
	Feats []string
	// Tier is the difficulty tier propagated from the template at spawn.
	// Empty string means "standard" is assumed.
	Tier string
	// Disposition is the runtime NPC disposition; initialized from template, may change.
	Disposition string
	// MotiveBonus is the +2 attack bonus granted by a motive crit fail; applied once then zeroed.
	MotiveBonus int
	// AbilityCooldowns maps operator ID → rounds remaining until usable again.
	// Nil at spawn; initialized lazily on first write in applyPlanLocked.
	AbilityCooldowns map[string]int
	// BossAbilityCooldowns maps boss ability ID → time after which it may fire again.
	// Initialized to an empty non-nil map at spawn. Nil-safe check is not required.
	BossAbilityCooldowns map[string]time.Time
	// NPCType is copied from the template at spawn.
	// "combat" = participates in normal combat; other values = non-combat NPC.
	NPCType string
	// NpcRole is copied from Template.NpcRole at spawn time.
	// Empty means combat NPC (no POI contribution).
	NpcRole string
	// Personality is copied from the template at spawn; drives flee/cower behavior.
	Personality string
	// Cowering is true when this NPC is in a cower state because combat started
	// in their room. While Cowering == true, the NPC does not respond to commands.
	// Cleared when combat in their room ends.
	Cowering bool
	// FactionID is the faction this instance belongs to; copied from template at spawn.
	// Empty string means no faction affiliation.
	FactionID string
	// AttackVerb is the verb used in combat attack narratives. Copied from template.
	// Empty string means the default verb ("attacks") will be used.
	AttackVerb string
	// Immobile prevents this NPC from patrolling or wandering. Copied from template.
	Immobile bool
	// CourageThreshold copied from template; NPC engages when ThreatScore <= this. REQ-NB-10.
	CourageThreshold int
	// FleeHPPct is the HP% below which the NPC flees combat. 0 = never flee.
	FleeHPPct int
	// WanderRadius is the max BFS hops from HomeRoomID during patrol. 0 = no movement.
	WanderRadius int
	// GrudgePlayerID is the ID of the last player to deal damage to this NPC.
	// Cleared to "" on respawn. REQ-NB-12.
	GrudgePlayerID string
	// ReturningHome is true when the NPC is moving back to its HomeRoom after combat.
	// Cleared when the NPC arrives at HomeRoom. REQ-NB-41.
	ReturningHome bool
	// HomeRoomBFS is the precomputed BFS distance map from HomeRoom to all zone rooms.
	// Populated at zone load. REQ-NB-38.
	HomeRoomBFS map[string]int
	// HomeRoomID is the resolved home room ID (from Template.HomeRoom or spawn room).
	HomeRoomID string
	// PlayerEnteredRoom is true for exactly one idle tick after a player enters the NPC's room.
	// REQ-NB-4.
	PlayerEnteredRoom bool
	// OnDamageTaken is true for exactly one idle tick in the round the NPC received damage.
	// REQ-NB-4.
	OnDamageTaken bool
	// PendingFlee is true when FleeHPPct threshold was crossed; resolved in applyPlanLocked.
	PendingFlee bool
	// PendingJoinCombatRoomID is non-empty when the NPC was recruited via call_for_help
	// and should join combat in the given room on the next tick.
	PendingJoinCombatRoomID string
	// ProtectedNPCName is the display name of the NPC this instance is defending.
	// Non-empty when the NPC joined combat via COMBATMSG-4f (protecting an ally).
	// Set by the call_for_help recruit logic when the recruiter has a name.
	ProtectedNPCName string
	// RestCost is the credit cost charged to a player for a motel rest at this NPC.
	// 0 means this NPC is not a motel NPC and does not offer rest (REQ-REST-8).
	RestCost int
}

// HasTag reports whether the given tag is present in the instance's tag list.
//
// Precondition: tag must be non-empty for a meaningful result.
// Postcondition: Returns true iff tag is present in Tags; false otherwise.
func (i *Instance) HasTag(tag string) bool {
	for _, t := range i.Tags {
		if t == tag {
			return true
		}
	}
	return false
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

// IsAnimal returns true when the instance's Type is "animal".
func (i *Instance) IsAnimal() bool { return i.Type == "animal" }

// IsRobot returns true when the instance's Type is "robot".
func (i *Instance) IsRobot() bool { return i.Type == "robot" }

// IsMachine returns true when the instance's Type is "machine".
func (i *Instance) IsMachine() bool { return i.Type == "machine" }

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
// xpCfg is optional; when non-nil, MaxHP is scaled by the tier multiplier.
// featRegistry is optional; when non-nil, the "tough" feat adds +5 HP after tier scaling.
//
// Precondition: id must be non-empty; tmpl must be non-nil; roomID must be non-empty.
// Postcondition: CurrentHP equals computed MaxHP; WeaponID and ArmorID are set from weighted roll.
func NewInstanceWithResolver(id string, tmpl *Template, roomID string, armorACBonus func(string) int, xpCfg *xp.XPConfig, featRegistry *ruleset.FeatRegistry) *Instance {
	weaponID := pickWeighted(tmpl.Weapon)
	armorID := pickWeighted(tmpl.Armor)
	ac := tmpl.AC
	if armorID != "" && armorACBonus != nil {
		ac += armorACBonus(armorID)
	}

	// Compute tier-scaled MaxHP.
	tier := tmpl.Tier
	if tier == "" {
		tier = "standard"
	}
	maxHP := tmpl.MaxHP
	if xpCfg != nil {
		if mult, ok := xpCfg.TierMultipliers[tier]; ok {
			maxHP = int(math.Ceil(float64(tmpl.MaxHP) * mult.HP))
		}
	}
	// Apply tough feat bonus (+5 HP) after tier multiplier.
	if featRegistry != nil {
		for _, featID := range tmpl.Feats {
			if featID == "tough" {
				if f, ok := featRegistry.Feat("tough"); ok && f.AllowNPC {
					maxHP += 5
				}
			}
		}
	}

	return &Instance{
		ID:            id,
		TemplateID:    tmpl.ID,
		Type:          tmpl.Type,
		Gender:        tmpl.Gender,
		name:          tmpl.Name,
		baseName:      tmpl.Name,
		Description:   tmpl.Description,
		RoomID:        roomID,
		CurrentHP:     maxHP,
		MaxHP:         maxHP,
		AC:            ac,
		Level:         tmpl.Level,
		Awareness:     tmpl.Awareness,
		Hustle:        tmpl.Hustle,
		AIDomain:    tmpl.AIDomain,
		Loot:        tmpl.Loot,
		SkillChecks: tmpl.SkillChecks,
		Resistances:   resolveResistances(tmpl),
		Weaknesses:    tmpl.Weaknesses,
		WeaponID:      weaponID,
		ArmorID:       armorID,
		UseCover:      tmpl.Combat.UseCover,
		Brutality:     tmpl.Abilities.Brutality,
		Quickness:     tmpl.Abilities.Quickness,
		Savvy:         tmpl.Abilities.Savvy,
		Flair:         tmpl.Abilities.Flair,
		SeductionProbability: tmpl.SeductionProbability,
		SeductionGender:      tmpl.SeductionGender,
		ToughnessRank: tmpl.ToughnessRank,
		HustleRank:    tmpl.HustleRank,
		CoolRank:      tmpl.CoolRank,
		RobPercent:       computeRobPercent(tmpl.RobMultiplier, tmpl.Level),
		Currency:         0,
		SenseAbilities: append([]string(nil), tmpl.SenseAbilities...),
		Tags:           append([]string(nil), tmpl.Tags...),
		Feats:                append([]string(nil), tmpl.Feats...),
		Tier:                 tmpl.Tier,
		BossAbilityCooldowns: make(map[string]time.Time),
		NPCType:          tmpl.NPCType,
		NpcRole:          tmpl.NpcRole,
		Personality:      tmpl.Personality,
		AttackVerb: tmpl.AttackVerb,
		Immobile:   tmpl.Immobile,
		// Cowering defaults to false (zero value).
		FactionID: tmpl.FactionID,
		Disposition: func() string {
			if tmpl.Disposition == "" {
				return "hostile"
			}
			return tmpl.Disposition
		}(),
		CourageThreshold: tmpl.CourageThreshold,
		FleeHPPct:        tmpl.FleeHPPct,
		WanderRadius:     tmpl.WanderRadius,
		HomeRoomID: func() string {
			if tmpl.HomeRoom != "" {
				return tmpl.HomeRoom
			}
			return roomID
		}(),
	}
}

// resolveResistances computes the effective resistance map for a new instance.
// Robots and machines receive default bleed+poison immunity (999), which
// template values override on a per-key basis.
//
// Precondition: tmpl must not be nil.
// Postcondition: Returns a non-nil map for robots/machines; otherwise returns tmpl.Resistances as-is.
func resolveResistances(tmpl *Template) map[string]int {
	if tmpl.IsRobot() || tmpl.IsMachine() {
		resistances := map[string]int{
			combat.DamageTypeBleed:  999,
			combat.DamageTypePoison: 999,
		}
		for k, v := range tmpl.Resistances {
			resistances[k] = v
		}
		return resistances
	}
	return tmpl.Resistances
}

// computeRobPercent calculates the rob percentage for an NPC at spawn time.
// Returns 0 if multiplier is 0 (NPC does not rob).
// Otherwise returns clamp((rand(5,20) + min(level,10)) * multiplier, 5.0, 30.0).
//
// Precondition: multiplier >= 0; level >= 1.
// Postcondition: returns 0 if multiplier == 0; returns value in [5.0, 30.0] otherwise.
func computeRobPercent(multiplier float64, level int) float64 {
	if multiplier == 0 {
		return 0
	}
	base := 5 + rand.Intn(16) // [5, 20]
	levelBonus := level
	if levelBonus > 10 {
		levelBonus = 10
	}
	raw := float64(base+levelBonus) * multiplier
	if raw < 5.0 {
		raw = 5.0
	}
	if raw > 30.0 {
		raw = 30.0
	}
	return raw
}

// NewInstance creates a live NPC instance from a template with no armor AC resolver.
// Use NewInstanceWithResolver when an inventory registry is available to apply AC bonuses.
//
// Precondition: id must be non-empty; tmpl must be non-nil; roomID must be non-empty.
// Postcondition: CurrentHP equals tmpl.MaxHP; WeaponID/ArmorID are set; AC is base only.
func NewInstance(id string, tmpl *Template, roomID string) *Instance {
	return NewInstanceWithResolver(id, tmpl, roomID, nil, nil, nil)
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
