package gameserver

import (
	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/crafting"
	"github.com/cory-johannsen/mud/internal/game/faction"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/quest"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/substance"
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
	FactionRepRepo         FactionRepRepository
	MaterialRepo           CharacterMaterialsRepository
	// QuestRepo persists player quest status and objective progress.
	// May be nil when the quest feature is not yet configured.
	QuestRepo quest.QuestRepository
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
	// SubstanceRegistry holds all known substance definitions.
	// REQ-AH-3: loaded at startup from content/substances/.
	SubstanceRegistry    *substance.Registry
	// FactionRegistry holds all faction definitions loaded at startup.
	// May be nil when the faction feature is not yet configured.
	FactionRegistry *faction.FactionRegistry
	// FactionConfig holds global faction economy parameters (rep costs, rep per service).
	// May be nil when the faction feature is not yet configured.
	FactionConfig *faction.FactionConfig
	// RecipeRegistry holds all crafting recipe definitions loaded at startup.
	// May be nil when the crafting feature is not yet configured.
	RecipeRegistry *crafting.RecipeRegistry
	// MaterialRegistry holds all crafting material definitions loaded at startup.
	// May be nil when the crafting feature is not yet configured.
	MaterialRegistry *crafting.MaterialRegistry
	// QuestRegistry holds all quest definitions loaded at startup.
	// May be nil when the quest feature is not yet configured.
	QuestRegistry quest.QuestRegistry
}

// HandlerDeps groups all handler dependencies for GameServiceServer.
type HandlerDeps struct {
	WorldHandler  *WorldHandler
	ChatHandler   *ChatHandler
	NPCHandler    *NPCHandler
	CombatHandler *CombatHandler
	ActionHandler *ActionHandler
}
