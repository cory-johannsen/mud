package inventory

import (
	"fmt"
	"strings"
)

// DC constants for the crafting check to affix a precious material (REQ-GA-9).
const (
	DCCommonStreetGrade    = 16
	DCCommonMilSpecGrade   = 21
	DCCommonGhostGrade     = 26
	DCUncommonStreetGrade  = 18
	DCUncommonMilSpecGrade = 23
	DCUncommonGhostGrade   = 28
	DCRareStreetGrade      = 20
	DCRareMilSpecGrade     = 25
	DCRareGhostGrade       = 30
)

// MaterialAppliesToWeapon and MaterialAppliesToArmor are the valid AppliesTo values.
const (
	MaterialAppliesToWeapon = "weapon"
	MaterialAppliesToArmor  = "armor"
)

// GradeDisplayNames maps GradeID to the player-facing grade name.
var GradeDisplayNames = map[string]string{
	"street_grade":   "Street Grade",
	"mil_spec_grade": "Mil-Spec Grade",
	"ghost_grade":    "Ghost Grade",
}

// MaterialDef holds the static definition of one material at one grade.
// Constructed at load time from the corresponding ItemDef YAML fields.
type MaterialDef struct {
	MaterialID string
	Name       string
	GradeID    string
	GradeName  string
	Tier       string
	AppliesTo  []string
}

// AttackContext carries per-hit context for evaluating material effects.
type AttackContext struct {
	TargetIsCyberAugmented bool
	TargetIsSupernatural   bool
	TargetIsLightAspected  bool
	TargetIsShadowAspected bool
	TargetIsMetalArmored   bool
	IsHit                  bool
	IsFirstHitThisCombat   bool
}

// MaterialEffectResult holds per-hit effect values produced by ApplyMaterialEffects.
type MaterialEffectResult struct {
	DamageBonus              int
	PersistentFireDmg        int
	PersistentColdDmg        int
	PersistentRadDmg         int
	PersistentBleedDmg       int
	TargetLosesAP            int
	TargetSpeedPenalty       int
	TargetFlatFooted         bool
	TargetDazzled            bool
	TargetBlinded            bool
	TargetSlowed             bool
	SuppressRegeneration     bool
	IgnoreMetalArmorAC       bool
	IgnoreAllArmorAC         bool
	IgnoreHardnessThreshold  int
	ApplyOnFireCondition     bool // set by thermite_lace:ghost_grade on hit
	ApplyIrradiatedCondition bool // set by rad_core:ghost_grade on hit
}

// PassiveMaterialSummary accumulates passive bonuses from affixed materials.
type PassiveMaterialSummary struct {
	CheckPenaltyReduction int
	NoCheckPenalty        bool // set by carbon_weave:ghost_grade; caller treats this as zero check penalty regardless of CheckPenaltyReduction
	SpeedBonus            int
	BulkReduction         int
	StealthBonus          int
	MetalDetectionImmune  bool
	SaveVsTechBonus       int
	SaveVsMentalBonus     int
	ConditionImmunities   []string
	InitiativeBonus       int
	TechAttackRollBonus   int
	FPOnRecalibrateBonus  int
	HardnessBonus         int
	ACVsEnergyBonus       int
	CarrierRadDmgPerHour  int
}

// MaterialSessionState tracks per-combat and per-day stateful material effect usage.
type MaterialSessionState struct {
	CombatUsed map[string]bool
	DailyUsed  map[string]int
}

// DCForMaterial returns the crafting DC for the given material tier and grade ID.
// Returns 0 for unknown tier/grade combinations.
func DCForMaterial(tier, gradeID string) int {
	switch tier {
	case "common":
		switch gradeID {
		case "street_grade":
			return DCCommonStreetGrade
		case "mil_spec_grade":
			return DCCommonMilSpecGrade
		case "ghost_grade":
			return DCCommonGhostGrade
		}
	case "uncommon":
		switch gradeID {
		case "street_grade":
			return DCUncommonStreetGrade
		case "mil_spec_grade":
			return DCUncommonMilSpecGrade
		case "ghost_grade":
			return DCUncommonGhostGrade
		}
	case "rare":
		switch gradeID {
		case "street_grade":
			return DCRareStreetGrade
		case "mil_spec_grade":
			return DCRareMilSpecGrade
		case "ghost_grade":
			return DCRareGhostGrade
		}
	}
	return 0
}

// ApplyMaterialEffects accumulates per-hit effects from all affixed materials.
// Pure function — no side effects. Stateful effects are NOT handled here.
//
// Precondition: affixed entries must be formatted "<material_id>:<grade_id>".
// Postcondition: returns the aggregated MaterialEffectResult.
func ApplyMaterialEffects(affixed []string, ctx AttackContext, reg *Registry) MaterialEffectResult {
	var result MaterialEffectResult
	for _, entry := range affixed {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			continue
		}
		def, ok := reg.Material(parts[0], parts[1])
		if !ok {
			continue
		}
		applyMaterialEffect(def, ctx, &result)
	}
	return result
}

// applyMaterialEffect applies one material's per-hit effects into result.
func applyMaterialEffect(def *MaterialDef, ctx AttackContext, result *MaterialEffectResult) {
	key := def.MaterialID + ":" + def.GradeID
	switch key {
	// Scrap Iron — disrupts cyber-augmented enemies
	case "scrap_iron:street_grade":
		if ctx.TargetIsCyberAugmented && ctx.IsHit {
			result.DamageBonus += 1
		}
	case "scrap_iron:mil_spec_grade":
		if ctx.TargetIsCyberAugmented && ctx.IsHit {
			result.DamageBonus += 2
			result.TargetLosesAP += 1
		}
	case "scrap_iron:ghost_grade":
		if ctx.TargetIsCyberAugmented && ctx.IsHit {
			result.DamageBonus += 4
			result.TargetFlatFooted = true
		}
	// Hollow Point — weakens supernatural entities
	case "hollow_point:street_grade":
		if ctx.TargetIsSupernatural && ctx.IsHit {
			result.DamageBonus += 1
		}
	case "hollow_point:mil_spec_grade":
		if ctx.TargetIsSupernatural && ctx.IsHit {
			result.DamageBonus += 2
			result.PersistentBleedDmg += 1
		}
	case "hollow_point:ghost_grade":
		if ctx.TargetIsSupernatural && ctx.IsHit {
			result.DamageBonus += 4
			result.SuppressRegeneration = true
		}
	// Carbide Alloy — weapon effects (armor effects are passive, in ComputePassiveMaterials)
	case "carbide_alloy:street_grade":
		result.IgnoreHardnessThreshold = materialMax(result.IgnoreHardnessThreshold, 0)
	case "carbide_alloy:mil_spec_grade":
		result.IgnoreHardnessThreshold = materialMax(result.IgnoreHardnessThreshold, 5)
	case "carbide_alloy:ghost_grade":
		result.IgnoreHardnessThreshold = materialMax(result.IgnoreHardnessThreshold, 10)
	// Thermite Lace — fire damage
	case "thermite_lace:street_grade":
		if ctx.IsHit {
			result.PersistentFireDmg += 1
		}
	case "thermite_lace:mil_spec_grade":
		if ctx.IsHit {
			result.PersistentFireDmg += 2
			result.DamageBonus += 1
		}
	case "thermite_lace:ghost_grade":
		if ctx.IsHit {
			result.PersistentFireDmg += 4
			result.DamageBonus += 2
			result.ApplyOnFireCondition = true
		}
	// Cryo-Gel — cold damage
	case "cryo_gel:street_grade":
		if ctx.IsHit {
			result.PersistentColdDmg += 1
		}
	case "cryo_gel:mil_spec_grade":
		if ctx.IsHit {
			result.PersistentColdDmg += 2
			result.TargetSpeedPenalty += 5
		}
	case "cryo_gel:ghost_grade":
		if ctx.IsHit {
			result.PersistentColdDmg += 4
			result.TargetSlowed = true
		}
	// Rad-Core — radiation damage (carrier damage is passive, handled by ComputePassiveMaterials)
	case "rad_core:street_grade":
		if ctx.IsHit {
			result.PersistentRadDmg += 1
		}
	case "rad_core:mil_spec_grade":
		if ctx.IsHit {
			result.PersistentRadDmg += 2
		}
	case "rad_core:ghost_grade":
		if ctx.IsHit {
			result.PersistentRadDmg += 4
			result.ApplyIrradiatedCondition = true
		}
	// Ghost Steel — ignores armor AC (stateful first-hit handled by caller via ctx)
	case "ghost_steel:street_grade":
		if ctx.IsHit && ctx.IsFirstHitThisCombat && ctx.TargetIsMetalArmored {
			result.IgnoreMetalArmorAC = true
		}
	case "ghost_steel:mil_spec_grade":
		if ctx.IsHit {
			result.IgnoreMetalArmorAC = true
			result.DamageBonus += 1
		}
	case "ghost_steel:ghost_grade":
		if ctx.IsHit {
			result.IgnoreAllArmorAC = true
			result.DamageBonus += 2
		}
	// Shadow Plate — harms light-aspected
	case "shadow_plate:street_grade":
		if ctx.TargetIsLightAspected && ctx.IsHit {
			result.DamageBonus += 1
		}
	case "shadow_plate:mil_spec_grade":
		if ctx.TargetIsLightAspected && ctx.IsHit {
			result.DamageBonus += 2
			result.TargetDazzled = true
		}
	case "shadow_plate:ghost_grade":
		if ctx.TargetIsLightAspected && ctx.IsHit {
			result.DamageBonus += 4
			result.TargetBlinded = true
		}
	// Radiance Plate — harms shadow-aspected
	case "radiance_plate:street_grade":
		if ctx.TargetIsShadowAspected && ctx.IsHit {
			result.DamageBonus += 1
		}
	case "radiance_plate:mil_spec_grade":
		if ctx.TargetIsShadowAspected && ctx.IsHit {
			result.DamageBonus += 2
			result.TargetDazzled = true
		}
	case "radiance_plate:ghost_grade":
		if ctx.TargetIsShadowAspected && ctx.IsHit {
			result.DamageBonus += 4
			result.TargetBlinded = true
		}
	}
}

// ComputePassiveMaterials accumulates passive bonuses from affixed materials on equipped items.
// Pure function. Called at login and whenever equipped items change.
//
// equipped: active preset weapons only ([]*EquippedWeapon{preset.MainHand, preset.OffHand})
// armor: all equipped armor slots (sess.Equipment.Armor)
func ComputePassiveMaterials(equipped []*EquippedWeapon, armor map[ArmorSlot]*SlottedItem, reg *Registry) PassiveMaterialSummary {
	var s PassiveMaterialSummary
	var immunities []string

	// Process weapon slots
	for _, ew := range equipped {
		if ew == nil {
			continue
		}
		for _, entry := range ew.AffixedMaterials {
			parts := strings.SplitN(entry, ":", 2)
			if len(parts) != 2 {
				continue
			}
			def, ok := reg.Material(parts[0], parts[1])
			if !ok {
				continue
			}
			applyPassiveWeapon(def, &s, &immunities)
		}
	}

	// Process armor slots
	for _, si := range armor {
		if si == nil {
			continue
		}
		for _, entry := range si.AffixedMaterials {
			parts := strings.SplitN(entry, ":", 2)
			if len(parts) != 2 {
				continue
			}
			def, ok := reg.Material(parts[0], parts[1])
			if !ok {
				continue
			}
			applyPassiveArmor(def, si, &s, &immunities)
		}
	}

	s.ConditionImmunities = immunities
	return s
}

func applyPassiveWeapon(def *MaterialDef, s *PassiveMaterialSummary, immunities *[]string) {
	key := def.MaterialID + ":" + def.GradeID
	switch key {
	case "null_weave:street_grade":
		s.SaveVsTechBonus += 1
	case "null_weave:mil_spec_grade":
		s.SaveVsTechBonus += 2
	case "null_weave:ghost_grade":
		s.SaveVsTechBonus += 3
	case "quantum_alloy:mil_spec_grade":
		s.InitiativeBonus += 1
	case "quantum_alloy:ghost_grade":
		s.InitiativeBonus += 2
	case "neural_gel:mil_spec_grade":
		s.TechAttackRollBonus = materialMax(s.TechAttackRollBonus, 1)
	case "neural_gel:ghost_grade":
		s.TechAttackRollBonus = materialMax(s.TechAttackRollBonus, 1)
		s.FPOnRecalibrateBonus += 1
	case "rad_core:street_grade":
		s.CarrierRadDmgPerHour += 1
	case "rad_core:mil_spec_grade":
		s.CarrierRadDmgPerHour += 2
	case "rad_core:ghost_grade":
		s.CarrierRadDmgPerHour += 3
	}
}

func applyPassiveArmor(def *MaterialDef, si *SlottedItem, s *PassiveMaterialSummary, immunities *[]string) {
	key := def.MaterialID + ":" + def.GradeID
	switch key {
	case "carbon_weave:street_grade":
		s.CheckPenaltyReduction += 1
	case "carbon_weave:mil_spec_grade":
		s.CheckPenaltyReduction += 2
		s.SpeedBonus += 5
	case "carbon_weave:ghost_grade":
		s.NoCheckPenalty = true
		s.SpeedBonus += 10
	case "polymer_frame:street_grade":
		s.BulkReduction += 1
	case "polymer_frame:mil_spec_grade":
		s.BulkReduction += 2
		s.StealthBonus += 1
	case "polymer_frame:ghost_grade":
		s.BulkReduction += 3
		s.StealthBonus += 2
		s.MetalDetectionImmune = true
	case "carbide_alloy:street_grade":
		s.HardnessBonus += 1
	case "carbide_alloy:mil_spec_grade":
		s.HardnessBonus += 2
	case "carbide_alloy:ghost_grade":
		s.HardnessBonus += 3
	case "null_weave:street_grade":
		s.SaveVsTechBonus += 1
	case "null_weave:mil_spec_grade":
		s.SaveVsTechBonus += 2
	case "null_weave:ghost_grade":
		s.SaveVsTechBonus += 3
	case "quantum_alloy:mil_spec_grade":
		s.InitiativeBonus += 1
	case "quantum_alloy:ghost_grade":
		s.InitiativeBonus += 2
	case "rad_core:street_grade":
		s.CarrierRadDmgPerHour += 1
	case "rad_core:mil_spec_grade":
		s.CarrierRadDmgPerHour += 2
		s.ACVsEnergyBonus += 1
	case "rad_core:ghost_grade":
		s.CarrierRadDmgPerHour += 3
		s.ACVsEnergyBonus += 1
	case "neural_gel:mil_spec_grade":
		s.TechAttackRollBonus = materialMax(s.TechAttackRollBonus, 1)
	case "neural_gel:ghost_grade":
		s.TechAttackRollBonus = materialMax(s.TechAttackRollBonus, 1)
		s.FPOnRecalibrateBonus += 1
	case "soul_guard_alloy:street_grade":
		s.SaveVsMentalBonus += 1
		*immunities = appendIfAbsent(*immunities, "frightened")
	case "soul_guard_alloy:mil_spec_grade":
		s.SaveVsMentalBonus += 2
		*immunities = appendIfAbsent(*immunities, "frightened")
		*immunities = appendIfAbsent(*immunities, "confused")
	case "soul_guard_alloy:ghost_grade":
		s.SaveVsMentalBonus += 3
		// "*mental" is a sentinel: grpc_service expands it to all ConditionDef with IsMentalCondition=true
		*immunities = appendIfAbsent(*immunities, "*mental")
	}
	_ = si // reserved for future extension
}

func appendIfAbsent(slice []string, val string) []string {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}

func materialMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// MaterialDefFromItemDef constructs a MaterialDef from a precious-material ItemDef.
// Returns an error if the ItemDef is not of KindPreciousMaterial.
func MaterialDefFromItemDef(d *ItemDef) (*MaterialDef, error) {
	if d.Kind != KindPreciousMaterial {
		return nil, fmt.Errorf("inventory: MaterialDefFromItemDef: item %q is not precious_material kind", d.ID)
	}
	gradeName, ok := GradeDisplayNames[d.GradeID]
	if !ok {
		return nil, fmt.Errorf("inventory: MaterialDefFromItemDef: unknown grade_id %q", d.GradeID)
	}
	return &MaterialDef{
		MaterialID: d.MaterialID,
		Name:       d.MaterialName,
		GradeID:    d.GradeID,
		GradeName:  gradeName,
		Tier:       d.MaterialTier,
		AppliesTo:  d.AppliesTo,
	}, nil
}
