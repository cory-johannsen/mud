package gameserver

import (
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/scripting"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/cory-johannsen/mud/internal/game/trap"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// newTestGameServiceServer is a test-only shim that accepts the legacy positional
// parameter list and forwards them to the refactored NewGameServiceServer using the
// StorageDeps / ContentDeps / HandlerDeps structs.
//
// Precondition: worldMgr and sessMgr must be non-nil.
// Postcondition: Returns a fully initialised GameServiceServer suitable for unit tests.
//
//nolint:unparam
func newTestGameServiceServer(
	worldMgr *world.Manager,
	sessMgr *session.Manager,
	cmdRegistry *command.Registry,
	worldHandler *WorldHandler,
	chatHandler *ChatHandler,
	logger *zap.Logger,
	charSaver CharacterSaver,
	diceRoller *dice.Roller,
	npcHandler *NPCHandler,
	npcMgr *npc.Manager,
	combatHandler *CombatHandler,
	scriptMgr *scripting.Manager,
	respawnMgr *npc.RespawnManager,
	floorMgr *inventory.FloorManager,
	roomEquipMgr *inventory.RoomEquipmentManager,
	automapRepo *postgres.AutomapRepository,
	invRegistry *inventory.Registry,
	accountAdmin AccountAdmin,
	calendar *GameCalendar,
	jobRegistry *ruleset.JobRegistry,
	condRegistry *condition.Registry,
	techRegistry *technology.Registry,
	hardwiredTechRepo HardwiredTechRepo,
	preparedTechRepo PreparedTechRepo,
	knownTechRepo KnownTechRepo,
	innateTechRepo InnateTechRepo,
	loadoutsDir string,
	allSkills []*ruleset.Skill,
	characterSkillsRepo CharacterSkillsRepository,
	characterProficienciesRepo CharacterProficienciesRepository,
	allFeats []*ruleset.Feat,
	featRegistry *ruleset.FeatRegistry,
	characterFeatsRepo CharacterFeatsRepo,
	allClassFeatures []*ruleset.ClassFeature,
	classFeatureRegistry *ruleset.ClassFeatureRegistry,
	characterClassFeaturesRepo CharacterClassFeaturesGetter,
	featureChoicesRepo CharacterFeatureChoicesRepository,
	charAbilityBoostsRepo postgres.CharacterAbilityBoostsRepository,
	archetypes map[string]*ruleset.Archetype,
	regions map[string]*ruleset.Region,
	mentalStateMgr *mentalstate.Manager,
	actionH *ActionHandler,
	spontaneousUsePoolRepo SpontaneousUsePoolRepo,
	wantedRepo *postgres.WantedRepository,
	trapMgr *trap.TrapManager,
	trapTemplates map[string]*trap.TrapTemplate,
) *GameServiceServer {
	storage := StorageDeps{
		CharRepo:               charSaver,
		AccountRepo:            accountAdmin,
		SkillsRepo:             characterSkillsRepo,
		ProficienciesRepo:      characterProficienciesRepo,
		FeatsRepo:              characterFeatsRepo,
		ClassFeaturesRepo:      characterClassFeaturesRepo,
		FeatureChoicesRepo:     featureChoicesRepo,
		AbilityBoostsRepo:      charAbilityBoostsRepo,
		HardwiredTechRepo:      hardwiredTechRepo,
		PreparedTechRepo:       preparedTechRepo,
		KnownTechRepo:    knownTechRepo,
		InnateTechRepo:         innateTechRepo,
		SpontaneousUsePoolRepo: spontaneousUsePoolRepo,
		WantedRepo:             wantedRepo,
		AutomapRepo:            automapRepo,
	}
	content := ContentDeps{
		WorldMgr:             worldMgr,
		NpcMgr:               npcMgr,
		RespawnMgr:           respawnMgr,
		InvRegistry:          invRegistry,
		FloorMgr:             floorMgr,
		RoomEquipMgr:         roomEquipMgr,
		TechRegistry:         techRegistry,
		CondRegistry:         condRegistry,
		AllSkills:            allSkills,
		AllFeats:             allFeats,
		ClassFeatures:        allClassFeatures,
		FeatRegistry:         featRegistry,
		ClassFeatureRegistry: classFeatureRegistry,
		JobRegistry:          jobRegistry,
		ArchetypeMap:         archetypes,
		RegionMap:            regions,
		ScriptMgr:            scriptMgr,
		DiceRoller:           diceRoller,
		MentalStateMgr:       mentalStateMgr,
		LoadoutsDir:          LoadoutsDir(loadoutsDir),
	}
	handlers := HandlerDeps{
		WorldHandler:  worldHandler,
		ChatHandler:   chatHandler,
		NPCHandler:    npcHandler,
		CombatHandler: combatHandler,
		ActionHandler: actionH,
	}
	svc := NewGameServiceServer(storage, content, handlers, sessMgr, cmdRegistry, calendar, logger)
	// Preserve legacy trapMgr / trapTemplates injected directly in some tests.
	if trapMgr != nil {
		svc.trapMgr = trapMgr
	}
	if trapTemplates != nil {
		svc.trapTemplates = trapTemplates
	}
	return svc
}
