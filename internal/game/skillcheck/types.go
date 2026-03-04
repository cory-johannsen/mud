package skillcheck

// CheckOutcome represents the 4-tier result of a skill check (matches combat system).
type CheckOutcome int

const (
	CritSuccess CheckOutcome = iota
	Success
	Failure
	CritFailure
)

// String returns the lowercase string representation used in YAML and Lua hooks.
// Postcondition: never returns empty string.
func (o CheckOutcome) String() string {
	switch o {
	case CritSuccess:
		return "crit_success"
	case Success:
		return "success"
	case Failure:
		return "failure"
	case CritFailure:
		return "crit_failure"
	default:
		return "unknown"
	}
}

// Effect describes a mechanical consequence applied after a skill check outcome.
type Effect struct {
	Type    string `yaml:"type"`    // "damage" | "condition" | "deny" | "reveal"
	Formula string `yaml:"formula"` // dice formula for "damage" (e.g., "1d4")
	ID      string `yaml:"id"`      // condition ID for "condition"
	Target  string `yaml:"target"`  // target ID for "reveal"
}

// Outcome pairs a player-facing message with an optional mechanical effect.
type Outcome struct {
	Message string  `yaml:"message"`
	Effect  *Effect `yaml:"effect,omitempty"`
}

// OutcomeMap holds one Outcome per tier.
type OutcomeMap struct {
	CritSuccess *Outcome `yaml:"crit_success"`
	Success     *Outcome `yaml:"success"`
	Failure     *Outcome `yaml:"failure"`
	CritFailure *Outcome `yaml:"crit_failure"`
}

// ForOutcome returns the Outcome for a given CheckOutcome tier, or nil if not defined.
// Precondition: none (nil is a valid return).
func (m OutcomeMap) ForOutcome(o CheckOutcome) *Outcome {
	switch o {
	case CritSuccess:
		return m.CritSuccess
	case Success:
		return m.Success
	case Failure:
		return m.Failure
	case CritFailure:
		return m.CritFailure
	default:
		return nil
	}
}

// TriggerDef defines a single skill check trigger declared in YAML.
type TriggerDef struct {
	Skill    string     `yaml:"skill"`   // skill ID (e.g., "parkour")
	DC       int        `yaml:"dc"`      // difficulty class
	Trigger  string     `yaml:"trigger"` // "on_enter" | "on_greet" | "on_use"
	Outcomes OutcomeMap `yaml:"outcomes"`
}

// CheckResult holds the full result of a resolved skill check.
type CheckResult struct {
	TriggerDef TriggerDef
	Roll       int          // raw d20 result (1-20)
	AbilityMod int          // ability modifier applied
	ProfBonus  int          // proficiency bonus from rank
	Total      int          // Roll + AbilityMod + ProfBonus
	Outcome    CheckOutcome
}
