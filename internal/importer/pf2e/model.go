package pf2e

// PF2ESpell represents a single spell from the PF2E compendium JSON format.
type PF2ESpell struct {
	ID     string      `json:"_id"`
	Name   string      `json:"name"`
	System SpellSystem `json:"system"`
}

// SpellSystem holds the mechanical and descriptive data for a spell.
type SpellSystem struct {
	Description SpellDescription            `json:"description"`
	Level       SpellLevel                  `json:"level"`
	Traits      SpellTraits                 `json:"traits"`
	Time        SpellTime                   `json:"time"`
	Range       SpellRange                  `json:"range"`
	Target      SpellTarget                 `json:"target"`
	Area        *SpellArea                  `json:"area"`
	Duration    SpellDuration               `json:"duration"`
	Defense     SpellDefense                `json:"defense"`
	Damage      map[string]SpellDamageEntry `json:"damage"`
}

// SpellDescription holds the spell's text description.
type SpellDescription struct {
	Value string `json:"value"`
}

// SpellLevel holds the spell's level.
type SpellLevel struct {
	Value int `json:"value"`
}

// SpellTraits holds the spell's trait tags and tradition list.
type SpellTraits struct {
	Value      []string `json:"value"`
	Traditions []string `json:"traditions"`
}

// SpellTime holds the casting action cost string.
type SpellTime struct {
	Value string `json:"value"`
}

// SpellRange holds the range string.
type SpellRange struct {
	Value string `json:"value"`
}

// SpellTarget holds the target description string.
type SpellTarget struct {
	Value string `json:"value"`
}

// SpellArea holds the area-of-effect data (nullable in JSON).
type SpellArea struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

// SpellDuration holds the duration string.
type SpellDuration struct {
	Value string `json:"value"`
}

// SpellDefense holds the defense/save data for a spell (nullable in JSON).
type SpellDefense struct {
	Save *SpellDefenseSave `json:"save"`
}

// SpellDefenseSave holds the save statistic for a defense-based spell.
type SpellDefenseSave struct {
	Basic     bool   `json:"basic"`
	Statistic string `json:"statistic"`
}

// SpellDamageEntry holds a single damage component.
type SpellDamageEntry struct {
	Formula    string `json:"formula"`
	DamageType string `json:"type"`
}
