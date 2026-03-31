package postgres

import (
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolDB unwraps the postgres Pool to the raw pgxpool.Pool required by repository constructors.
func PoolDB(p *Pool) *pgxpool.Pool {
	return p.DB()
}

// StorageProviders is the wire provider set for all storage dependencies.
var StorageProviders = wire.NewSet(
	NewPool,
	PoolDB,
	NewCharacterRepository,
	NewAccountRepository,
	NewCharacterSkillsRepository,
	NewCharacterProficienciesRepository,
	NewCharacterFeatsRepository,
	NewCharacterClassFeaturesRepository,
	NewCharacterFeatureChoicesRepo,
	NewCharacterAbilityBoostsRepository,
	NewCharacterHardwiredTechRepository,
	NewCharacterPreparedTechRepository,
	NewCharacterSpontaneousTechRepository,
	NewCharacterInnateTechRepository,
	NewCharacterSpontaneousUsePoolRepository,
	NewWantedRepository,
	NewAutomapRepository,
	NewCalendarRepo,
	NewCharacterProgressRepository,
	NewQuestRepository,
	NewCharacterDowntimeRepository,
	NewWeatherRepo,
	wire.Bind(new(CharacterAbilityBoostsRepository), new(*PostgresCharacterAbilityBoostsRepository)),
)
