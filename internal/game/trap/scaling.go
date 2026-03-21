package trap

// ScalingBonuses holds the additive bonuses from danger level scaling.
// All bonuses are applied at trigger/detection time (REQ-TR-8).
type ScalingBonuses struct {
	// DamageDice is an additional dice expression to add to base damage (e.g. "1d6").
	// Empty string means no damage bonus.
	DamageDice     string
	SaveDCBonus    int
	StealthDCBonus int
	DisableDCBonus int
}

// globalDefaults maps danger level to the baseline ScalingBonuses used when
// the trap template has no per-template override for that tier.
var globalDefaults = map[string]ScalingBonuses{
	"safe":        {},
	"sketchy":     {},
	"dangerous":   {DamageDice: "1d6", SaveDCBonus: 2, StealthDCBonus: 2, DisableDCBonus: 2},
	"all_out_war": {DamageDice: "2d6", SaveDCBonus: 4, StealthDCBonus: 4, DisableDCBonus: 4},
}

// ScalingFor returns the ScalingBonuses for tmpl at dangerLevel.
// Per-template overrides in tmpl.DangerScaling take precedence over globalDefaults.
// Unknown danger levels return zero bonuses.
//
// Precondition: tmpl must be non-nil.
// Postcondition: Always returns a valid ScalingBonuses (never errors).
func ScalingFor(tmpl *TrapTemplate, dangerLevel string) ScalingBonuses {
	defaults := globalDefaults[dangerLevel] // zero value if unknown

	if tmpl.DangerScaling == nil {
		return defaults
	}

	var override *DangerScalingEntry
	switch dangerLevel {
	case "sketchy":
		override = tmpl.DangerScaling.Sketchy
	case "dangerous":
		override = tmpl.DangerScaling.Dangerous
	case "all_out_war":
		override = tmpl.DangerScaling.AllOutWar
	}

	if override == nil {
		return defaults
	}

	return ScalingBonuses{
		DamageDice:     override.DamageBonus,
		SaveDCBonus:    override.SaveDCBonus,
		StealthDCBonus: override.StealthDCBonus,
		DisableDCBonus: override.DisableDCBonus,
	}
}
