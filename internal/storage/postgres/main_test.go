package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/cory-johannsen/mud/internal/config"
	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

// sharedPool is the single database pool shared across all tests in this package.
// It is initialized by TestMain before any test runs.
var sharedPool *pgxpool.Pool

// TestMain starts one Postgres container for the entire package, applies all
// migrations, runs the tests, and terminates the container.
//
// Precondition: Docker must be available.
// Postcondition: All tests run against a single, fully-migrated database.
func TestMain(m *testing.M) {
	ctx := context.Background()
	start := time.Now()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "test",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting postgres container: %v [%s]\n", err, time.Since(start))
		os.Exit(1)
	}

	host, err := container.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "getting container host: %v\n", err)
		os.Exit(1)
	}
	mappedPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		fmt.Fprintf(os.Stderr, "getting mapped port: %v\n", err)
		os.Exit(1)
	}

	dbCfg := config.DatabaseConfig{
		Host:            host,
		Port:            mappedPort.Int(),
		User:            "test",
		Password:        "test",
		Name:            "test",
		SSLMode:         "disable",
		MaxConns:        10,
		MinConns:        1,
		MaxConnLifetime: 5 * time.Minute,
	}

	pool, err := pgstore.NewPool(ctx, dbCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connecting to test postgres: %v [%s]\n", err, time.Since(start))
		os.Exit(1)
	}
	sharedPool = pool.DB()

	if err := applyAllMigrations(sharedPool); err != nil {
		fmt.Fprintf(os.Stderr, "applying migrations: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "postgres container ready [%s]\n", time.Since(start))

	code := m.Run()

	pool.Close()
	termCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = container.Terminate(termCtx)

	os.Exit(code)
}

// applyAllMigrations applies all schema migrations needed by the postgres_test package.
//
// Precondition: pool must be connected.
// Postcondition: All tables used by integration tests exist.
func applyAllMigrations(pool *pgxpool.Pool) error {
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS accounts (
			id            BIGSERIAL    PRIMARY KEY,
			username      VARCHAR(64)  NOT NULL UNIQUE,
			password_hash TEXT         NOT NULL,
			role          VARCHAR(16)  NOT NULL DEFAULT 'player',
			banned        BOOLEAN      NOT NULL DEFAULT false,
			created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_accounts_username ON accounts (username);

		CREATE TABLE IF NOT EXISTS characters (
			id              BIGSERIAL    PRIMARY KEY,
			account_id      BIGINT       NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			name            VARCHAR(64)  NOT NULL,
			region          TEXT         NOT NULL,
			class           TEXT         NOT NULL,
			team            TEXT         NOT NULL DEFAULT '',
			level           INT          NOT NULL DEFAULT 1,
			experience      INT          NOT NULL DEFAULT 0,
			location        TEXT         NOT NULL DEFAULT 'battle_infirmary',
			brutality       INT          NOT NULL DEFAULT 10,
			quickness       INT          NOT NULL DEFAULT 10,
			grit            INT          NOT NULL DEFAULT 10,
			reasoning       INT          NOT NULL DEFAULT 10,
			savvy           INT          NOT NULL DEFAULT 10,
			flair           INT          NOT NULL DEFAULT 10,
			max_hp          INT          NOT NULL DEFAULT 8,
			current_hp      INT          NOT NULL DEFAULT 8,
			created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_characters_account_name UNIQUE (account_id, name)
		);
		CREATE INDEX IF NOT EXISTS idx_characters_account_id ON characters (account_id);

		ALTER TABLE characters
			ADD COLUMN IF NOT EXISTS has_received_starting_inventory BOOLEAN NOT NULL DEFAULT FALSE;

		CREATE TABLE IF NOT EXISTS character_weapon_presets (
			id           BIGSERIAL PRIMARY KEY,
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			preset_index INT NOT NULL,
			slot         TEXT NOT NULL,
			item_def_id  TEXT NOT NULL,
			ammo_count   INT NOT NULL DEFAULT 0,
			CONSTRAINT uq_character_preset_slot UNIQUE (character_id, preset_index, slot)
		);

		CREATE TABLE IF NOT EXISTS character_equipment (
			id           BIGSERIAL PRIMARY KEY,
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			slot         TEXT NOT NULL,
			item_def_id  TEXT NOT NULL,
			CONSTRAINT uq_character_equipment_slot UNIQUE (character_id, slot)
		);

		CREATE TABLE IF NOT EXISTS character_inventory (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			item_def_id  TEXT   NOT NULL,
			quantity     INT    NOT NULL DEFAULT 1,
			PRIMARY KEY (character_id, item_def_id)
		);

		CREATE TABLE IF NOT EXISTS character_map_rooms (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			zone_id      TEXT   NOT NULL,
			room_id      TEXT   NOT NULL,
			explored     BOOLEAN NOT NULL DEFAULT TRUE,
			PRIMARY KEY (character_id, zone_id, room_id)
		);

		CREATE TABLE IF NOT EXISTS character_skills (
			character_id BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			skill_id     TEXT    NOT NULL,
			proficiency  TEXT    NOT NULL DEFAULT 'untrained',
			PRIMARY KEY (character_id, skill_id)
		);

		CREATE TABLE IF NOT EXISTS character_feats (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			feat_id      TEXT NOT NULL,
			PRIMARY KEY (character_id, feat_id)
		);

		CREATE TABLE IF NOT EXISTS character_feature_choices (
			character_id  BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			feature_id    TEXT    NOT NULL,
			choice_key    TEXT    NOT NULL,
			value         TEXT    NOT NULL,
			PRIMARY KEY (character_id, feature_id, choice_key)
		);

		CREATE TABLE IF NOT EXISTS character_ability_boosts (
			character_id  BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			source        TEXT    NOT NULL,
			ability       TEXT    NOT NULL,
			PRIMARY KEY (character_id, source, ability)
		);

		CREATE TABLE IF NOT EXISTS character_proficiencies (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			category     TEXT   NOT NULL,
			rank         TEXT   NOT NULL DEFAULT 'untrained',
			PRIMARY KEY (character_id, category)
		);

		CREATE TABLE IF NOT EXISTS character_pending_boosts (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			count        INT    NOT NULL DEFAULT 0,
			PRIMARY KEY (character_id)
		);

		ALTER TABLE characters ADD COLUMN IF NOT EXISTS default_combat_action TEXT NOT NULL DEFAULT 'attack';

		ALTER TABLE characters ADD COLUMN IF NOT EXISTS pending_skill_increases INTEGER NOT NULL DEFAULT 0;

		ALTER TABLE characters ADD COLUMN IF NOT EXISTS gender TEXT NOT NULL DEFAULT '';

		ALTER TABLE characters ADD COLUMN IF NOT EXISTS skill_increases_initialized BOOLEAN NOT NULL DEFAULT FALSE;

		ALTER TABLE characters ADD COLUMN IF NOT EXISTS currency INTEGER NOT NULL DEFAULT 0;

		ALTER TABLE characters ADD COLUMN IF NOT EXISTS hero_points INTEGER NOT NULL DEFAULT 0;

		CREATE TABLE IF NOT EXISTS character_hardwired_technologies (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			tech_id      TEXT   NOT NULL,
			PRIMARY KEY (character_id, tech_id)
		);

		CREATE TABLE IF NOT EXISTS character_prepared_technologies (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			slot_level   INT    NOT NULL,
			slot_index   INT    NOT NULL,
			tech_id      TEXT   NOT NULL,
			expended     BOOLEAN NOT NULL DEFAULT FALSE,
			PRIMARY KEY (character_id, slot_level, slot_index)
		);

		CREATE TABLE IF NOT EXISTS character_spontaneous_technologies (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			tech_id      TEXT   NOT NULL,
			level        INT    NOT NULL,
			PRIMARY KEY (character_id, tech_id)
		);

		CREATE TABLE IF NOT EXISTS character_innate_technologies (
			character_id   BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			tech_id        TEXT   NOT NULL,
			max_uses       INT    NOT NULL DEFAULT 0,
			uses_remaining INT    NOT NULL DEFAULT 0,
			PRIMARY KEY (character_id, tech_id)
		);

		CREATE TABLE IF NOT EXISTS character_spontaneous_use_pools (
			character_id   BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			tech_level     INT    NOT NULL,
			uses_remaining INT    NOT NULL DEFAULT 0,
			max_uses       INT    NOT NULL DEFAULT 0,
			PRIMARY KEY (character_id, tech_level)
		);

		CREATE TABLE IF NOT EXISTS world_calendar (
			id    INTEGER PRIMARY KEY DEFAULT 1,
			day   INTEGER NOT NULL,
			month INTEGER NOT NULL,
			hour  INTEGER NOT NULL DEFAULT 6
		);

		-- Migration 056
		ALTER TABLE world_calendar ADD COLUMN IF NOT EXISTS tick BIGINT NOT NULL DEFAULT 0;

		CREATE TABLE IF NOT EXISTS character_wanted_levels (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			zone_id      VARCHAR(64) NOT NULL,
			wanted_level INTEGER NOT NULL CHECK (wanted_level BETWEEN 1 AND 4),
			PRIMARY KEY (character_id, zone_id)
		);

		ALTER TABLE characters ADD COLUMN IF NOT EXISTS detained_until TIMESTAMPTZ;

		ALTER TABLE character_equipment
			ADD COLUMN IF NOT EXISTS durability     INT NOT NULL DEFAULT 0,
			ADD COLUMN IF NOT EXISTS max_durability INT NOT NULL DEFAULT 0,
			ADD COLUMN IF NOT EXISTS modifier       TEXT NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS curse_revealed BOOLEAN NOT NULL DEFAULT FALSE,
			ADD COLUMN IF NOT EXISTS rarity         TEXT NOT NULL DEFAULT 'street';

		ALTER TABLE character_weapon_presets
			ADD COLUMN IF NOT EXISTS durability     INT NOT NULL DEFAULT 0,
			ADD COLUMN IF NOT EXISTS max_durability INT NOT NULL DEFAULT 0,
			ADD COLUMN IF NOT EXISTS modifier       TEXT NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS curse_revealed BOOLEAN NOT NULL DEFAULT FALSE,
			ADD COLUMN IF NOT EXISTS rarity         TEXT NOT NULL DEFAULT 'street';

		ALTER TABLE characters ADD COLUMN IF NOT EXISTS team TEXT NOT NULL DEFAULT '';

		CREATE TABLE IF NOT EXISTS character_inventory_instances (
			instance_id    TEXT PRIMARY KEY,
			character_id   BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			item_def_id    TEXT NOT NULL,
			durability     INT NOT NULL DEFAULT -1,
			max_durability INT NOT NULL DEFAULT -1,
			modifier       TEXT NOT NULL DEFAULT '',
			curse_revealed BOOLEAN NOT NULL DEFAULT FALSE,
			rarity         TEXT NOT NULL DEFAULT 'street',
			charges_remaining INTEGER NOT NULL DEFAULT -1,
			expended          BOOLEAN NOT NULL DEFAULT FALSE
		);

		-- Migration 040
		ALTER TABLE characters ADD COLUMN IF NOT EXISTS faction_id TEXT NOT NULL DEFAULT '';

		-- Migration 041
		CREATE TABLE IF NOT EXISTS character_faction_rep (
			character_id BIGINT NOT NULL REFERENCES characters(id),
			faction_id   TEXT NOT NULL,
			rep          INT NOT NULL DEFAULT 0,
			PRIMARY KEY (character_id, faction_id)
		);

		-- Migration 042
		ALTER TABLE characters ADD COLUMN IF NOT EXISTS focus_points INT NOT NULL DEFAULT 0;

		-- Migration 043
		CREATE TABLE IF NOT EXISTS character_materials (
			character_id  bigint NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			material_id   text   NOT NULL,
			quantity      int    NOT NULL CHECK (quantity > 0),
			PRIMARY KEY (character_id, material_id)
		);

		-- Migration 047
		CREATE TABLE IF NOT EXISTS character_quests (
			character_id BIGINT NOT NULL REFERENCES characters(id),
			quest_id     TEXT   NOT NULL,
			status       TEXT   NOT NULL,
			completed_at TIMESTAMPTZ,
			PRIMARY KEY (character_id, quest_id)
		);

		CREATE TABLE IF NOT EXISTS character_quest_progress (
			character_id BIGINT NOT NULL REFERENCES characters(id),
			quest_id     TEXT   NOT NULL,
			objective_id TEXT   NOT NULL,
			progress     INT    NOT NULL DEFAULT 0,
			PRIMARY KEY (character_id, quest_id, objective_id)
		);

		-- Migration 048
		CREATE TABLE IF NOT EXISTS character_downtime (
			character_id      bigint PRIMARY KEY REFERENCES characters(id) ON DELETE CASCADE,
			activity_id       text        NOT NULL,
			completes_at      timestamptz NOT NULL,
			room_id           text        NOT NULL,
			activity_metadata jsonb
		);

		-- Migration 049
		CREATE TABLE IF NOT EXISTS character_downtime_queue (
			id            bigserial PRIMARY KEY,
			character_id  bigint NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			position      int    NOT NULL,
			activity_id   text   NOT NULL,
			activity_args text,
			UNIQUE (character_id, position)
		);

		-- Migration 051
		CREATE TABLE IF NOT EXISTS character_jobs (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			job_id       TEXT   NOT NULL,
			PRIMARY KEY (character_id, job_id)
		);

		-- Migration 053
		ALTER TABLE characters ADD COLUMN IF NOT EXISTS hotbar TEXT;

		-- Migration 057
		CREATE TABLE IF NOT EXISTS weather_events (
			id                SERIAL PRIMARY KEY,
			weather_type      TEXT   NOT NULL,
			end_tick          BIGINT NOT NULL,
			cooldown_end_tick BIGINT NOT NULL DEFAULT 0,
			active            BOOL   NOT NULL DEFAULT TRUE
		);
		CREATE UNIQUE INDEX IF NOT EXISTS weather_events_one_active
			ON weather_events (active)
			WHERE active = TRUE;
	`)
	return err
}
