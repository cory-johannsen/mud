package ruleset

// CastingModel describes how a character acquires and uses technologies,
// mirroring PF2E spellcasting class mechanics.
type CastingModel string

const (
	// CastingModelWizard mirrors the PF2E Wizard/Witch: prepared caster with a catalog
	// (KnownTechs). Gains 2 extra catalog entries at L1 and +2 per subsequent level-up.
	CastingModelWizard CastingModel = "wizard"

	// CastingModelDruid mirrors the PF2E Druid/Cleric: prepared caster with access to
	// the full grant pool at every rest. No catalog tracking.
	CastingModelDruid CastingModel = "druid"

	// CastingModelRanger mirrors the PF2E Ranger: prepared caster whose catalog is
	// exactly the techs assigned at level-up and via trainers. No per-level extras.
	CastingModelRanger CastingModel = "ranger"

	// CastingModelSpontaneous mirrors the PF2E Bard/Sorcerer: knows a fixed set of
	// techs (KnownTechs) and casts from a shared use pool. Existing behavior unchanged.
	CastingModelSpontaneous CastingModel = "spontaneous"

	// CastingModelNone indicates no technology system. Used by Aggressor and Criminal.
	CastingModelNone CastingModel = "none"
)

// ResolveCastingModel returns the effective casting model for a character.
// Postcondition: job.CastingModel overrides archetype.CastingModel when non-empty and non-none;
// CastingModelNone is returned when both are unset.
func ResolveCastingModel(job *Job, archetype *Archetype) CastingModel {
	if job != nil && job.CastingModel != "" && job.CastingModel != CastingModelNone {
		return job.CastingModel
	}
	if archetype != nil && archetype.CastingModel != "" {
		return archetype.CastingModel
	}
	return CastingModelNone
}
