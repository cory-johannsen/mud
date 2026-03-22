package gameserver

import (
	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/scripting"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// LoadoutsDir is a named string type for the loadouts directory path.
// Named to avoid wire ambiguity with other string-typed paths.
type LoadoutsDir string

// StorageDeps groups all repository dependencies for GameServiceServer.
type StorageDeps struct {
	CharRepo               CharacterSaver
	AccountRepo            AccountAdmin
	SkillsRepo             CharacterSkillsRepository
	ProficienciesRepo      CharacterProficienciesRepository
	FeatsRepo              CharacterFeatsGetter
	ClassFeaturesRepo      CharacterClassFeaturesGetter
	FeatureChoicesRepo     CharacterFeatureChoicesRepository
	AbilityBoostsRepo      postgres.CharacterAbilityBoostsRepository
	HardwiredTechRepo      HardwiredTechRepo
	PreparedTechRepo       PreparedTechRepo
	SpontaneousTechRepo    SpontaneousTechRepo
	InnateTechRepo         InnateTechRepo
	SpontaneousUsePoolRepo SpontaneousUsePoolRepo
	WantedRepo             *postgres.WantedRepository
	AutomapRepo            *postgres.AutomapRepository
	DetainedUntilRepo      DetainedUntilUpdater
}

// ContentDeps groups all content/world dependencies for GameServiceServer.
type ContentDeps struct {
	WorldMgr             *world.Manager
	NpcMgr               *npc.Manager
	RespawnMgr           *npc.RespawnManager
	InvRegistry          *inventory.Registry
	FloorMgr             *inventory.FloorManager
	RoomEquipMgr         *inventory.RoomEquipmentManager
	TechRegistry         *technology.Registry
	CondRegistry         *condition.Registry
	AIRegistry           *ai.Registry
	AllSkills            []*ruleset.Skill
	AllFeats             []*ruleset.Feat
	ClassFeatures        []*ruleset.ClassFeature
	FeatRegistry         *ruleset.FeatRegistry
	ClassFeatureRegistry *ruleset.ClassFeatureRegistry
	JobRegistry          *ruleset.JobRegistry
	ArchetypeMap         map[string]*ruleset.Archetype
	RegionMap            map[string]*ruleset.Region
	ScriptMgr            *scripting.Manager
	DiceRoller           *dice.Roller
	CombatEngine         *combat.Engine
	MentalStateMgr       *mentalstate.Manager
	LoadoutsDir          LoadoutsDir
	// SetRegistry holds equipment set definitions for computing set bonuses (REQ-EM-29/35).
	// May be nil (treated as empty — no set bonuses).
	SetRegistry          *inventory.SetRegistry
}

// HandlerDeps groups all handler dependencies for GameServiceServer.
type HandlerDeps struct {
	WorldHandler  *WorldHandler
	ChatHandler   *ChatHandler
	NPCHandler    *NPCHandler
	CombatHandler *CombatHandler
	ActionHandler *ActionHandler
}
