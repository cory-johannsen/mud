package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// ErrCharacterNotFound is returned when a character lookup yields no results.
var ErrCharacterNotFound = errors.New("character not found")

// ErrCharacterNameTaken is returned when creating a character with a name already used by the account.
var ErrCharacterNameTaken = errors.New("character name already taken")

// CharacterRepository provides character persistence operations.
type CharacterRepository struct {
	db *pgxpool.Pool
}

// NewCharacterRepository creates a CharacterRepository backed by the given pool.
//
// Precondition: db must be a valid, open connection pool.
func NewCharacterRepository(db *pgxpool.Pool) *CharacterRepository {
	return &CharacterRepository{db: db}
}

// Pool returns the underlying connection pool.
//
// Postcondition: Returns the pool passed to NewCharacterRepository.
func (r *CharacterRepository) Pool() *pgxpool.Pool {
	return r.db
}

// Create inserts a new character and returns it with ID and timestamps set.
//
// Precondition: c.AccountID must reference an existing account; c.Name must be non-empty.
// Postcondition: Returns the created character with ID set, or ErrCharacterNameTaken on duplicate.
func (r *CharacterRepository) Create(ctx context.Context, c *character.Character) (*character.Character, error) {
	var out character.Character
	err := r.db.QueryRow(ctx, `
		INSERT INTO characters
			(account_id, name, region, class, team, level, experience, location,
			 brutality, quickness, grit, reasoning, savvy, flair,
			 max_hp, current_hp, gender, faction_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		RETURNING id, account_id, name, region, class, team, level, experience, location,
		          brutality, quickness, grit, reasoning, savvy, flair,
		          max_hp, current_hp, created_at, updated_at, default_combat_action, gender, faction_id`,
		c.AccountID, c.Name, c.Region, c.Class, c.Team, c.Level, c.Experience, c.Location,
		c.Abilities.Brutality, c.Abilities.Quickness, c.Abilities.Grit,
		c.Abilities.Reasoning, c.Abilities.Savvy, c.Abilities.Flair,
		c.MaxHP, c.CurrentHP, c.Gender, c.FactionID,
	).Scan(
		&out.ID, &out.AccountID, &out.Name, &out.Region, &out.Class, &out.Team,
		&out.Level, &out.Experience, &out.Location,
		&out.Abilities.Brutality, &out.Abilities.Quickness, &out.Abilities.Grit,
		&out.Abilities.Reasoning, &out.Abilities.Savvy, &out.Abilities.Flair,
		&out.MaxHP, &out.CurrentHP, &out.CreatedAt, &out.UpdatedAt, &out.DefaultCombatAction, &out.Gender, &out.FactionID,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrCharacterNameTaken
		}
		return nil, fmt.Errorf("inserting character: %w", err)
	}
	return &out, nil
}

// ListByAccount returns all characters for the given account ID, ordered by created_at.
//
// Precondition: accountID must be > 0.
// Postcondition: Returns a slice (may be empty) or a non-nil error.
func (r *CharacterRepository) ListByAccount(ctx context.Context, accountID int64) ([]*character.Character, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, account_id, name, region, class, team, level, experience, location,
		       brutality, quickness, grit, reasoning, savvy, flair,
		       max_hp, current_hp, created_at, updated_at, default_combat_action, gender, faction_id
		FROM characters WHERE account_id = $1 ORDER BY created_at ASC`,
		accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing characters: %w", err)
	}
	defer rows.Close()

	chars := make([]*character.Character, 0)
	for rows.Next() {
		var c character.Character
		if err := rows.Scan(
			&c.ID, &c.AccountID, &c.Name, &c.Region, &c.Class, &c.Team,
			&c.Level, &c.Experience, &c.Location,
			&c.Abilities.Brutality, &c.Abilities.Quickness, &c.Abilities.Grit,
			&c.Abilities.Reasoning, &c.Abilities.Savvy, &c.Abilities.Flair,
			&c.MaxHP, &c.CurrentHP, &c.CreatedAt, &c.UpdatedAt, &c.DefaultCombatAction, &c.Gender, &c.FactionID,
		); err != nil {
			return nil, fmt.Errorf("scanning character row: %w", err)
		}
		chars = append(chars, &c)
	}
	return chars, rows.Err()
}

// GetByID retrieves a character by its primary key.
//
// Precondition: id must be > 0.
// Postcondition: Returns the Character or ErrCharacterNotFound.
func (r *CharacterRepository) GetByID(ctx context.Context, id int64) (*character.Character, error) {
	var c character.Character
	err := r.db.QueryRow(ctx, `
		SELECT id, account_id, name, region, class, team, level, experience, location,
		       brutality, quickness, grit, reasoning, savvy, flair,
		       max_hp, current_hp, created_at, updated_at, default_combat_action, gender,
		       detained_until, faction_id
		FROM characters WHERE id = $1`,
		id,
	).Scan(
		&c.ID, &c.AccountID, &c.Name, &c.Region, &c.Class, &c.Team,
		&c.Level, &c.Experience, &c.Location,
		&c.Abilities.Brutality, &c.Abilities.Quickness, &c.Abilities.Grit,
		&c.Abilities.Reasoning, &c.Abilities.Savvy, &c.Abilities.Flair,
		&c.MaxHP, &c.CurrentHP, &c.CreatedAt, &c.UpdatedAt, &c.DefaultCombatAction, &c.Gender,
		&c.DetainedUntil, &c.FactionID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCharacterNotFound
		}
		return nil, fmt.Errorf("querying character: %w", err)
	}
	return &c, nil
}

// UpdateDetainedUntil persists the detained_until timestamp for a character.
// Pass nil to clear the detention.
//
// Precondition: characterID must be > 0.
// Postcondition: characters.detained_until is updated; returns ErrCharacterNotFound if no row updated.
func (r *CharacterRepository) UpdateDetainedUntil(ctx context.Context, characterID int64, detainedUntil *time.Time) error {
	if characterID <= 0 {
		return fmt.Errorf("UpdateDetainedUntil: characterID must be > 0, got %d", characterID)
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE characters SET detained_until = $2 WHERE id = $1`,
		characterID, detainedUntil,
	)
	if err != nil {
		return fmt.Errorf("UpdateDetainedUntil: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("UpdateDetainedUntil: %w", ErrCharacterNotFound)
	}
	return nil
}

// SaveDefaultCombatAction persists the player's preferred default combat action.
//
// Precondition: characterID > 0; action must be non-empty.
// Postcondition: characters.default_combat_action updated for the given ID.
func (r *CharacterRepository) SaveDefaultCombatAction(ctx context.Context, characterID int64, action string) error {
	if characterID <= 0 {
		return fmt.Errorf("SaveDefaultCombatAction: characterID must be > 0")
	}
	if action == "" {
		return fmt.Errorf("SaveDefaultCombatAction: action must be non-empty")
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE characters SET default_combat_action = $2 WHERE id = $1`,
		characterID, action,
	)
	if err != nil {
		return fmt.Errorf("SaveDefaultCombatAction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("SaveDefaultCombatAction: %w", ErrCharacterNotFound)
	}
	return nil
}

// SaveGender persists the player's gender to the characters table.
//
// Precondition: id > 0; gender must be non-empty.
// Postcondition: characters.gender column is updated; returns ErrCharacterNotFound if no row updated.
func (r *CharacterRepository) SaveGender(ctx context.Context, id int64, gender string) error {
	if id <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", id)
	}
	if gender == "" {
		return fmt.Errorf("gender must be non-empty")
	}
	tag, err := r.db.Exec(ctx, `UPDATE characters SET gender = $2 WHERE id = $1`, id, gender)
	if err != nil {
		return fmt.Errorf("SaveGender: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCharacterNotFound
	}
	return nil
}

// SaveState persists a character's current location and HP after a session.
//
// Precondition: id must be > 0; location must be a valid room ID.
// Postcondition: Returns nil on success, ErrCharacterNotFound if no row updated.
func (r *CharacterRepository) SaveState(ctx context.Context, id int64, location string, currentHP int) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE characters SET location = $2, current_hp = $3, updated_at = NOW()
		WHERE id = $1`,
		id, location, currentHP,
	)
	if err != nil {
		return fmt.Errorf("saving character state: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCharacterNotFound
	}
	return nil
}

// SaveAbilities persists the six ability scores for a character.
//
// Precondition: characterID must be > 0.
// Postcondition: Returns nil on success, ErrCharacterNotFound if no row was updated.
func (r *CharacterRepository) SaveAbilities(ctx context.Context, characterID int64, abilities character.AbilityScores) error {
	if characterID <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", characterID)
	}
	tag, err := r.db.Exec(ctx, `
		UPDATE characters SET
			brutality = $2,
			grit = $3,
			quickness = $4,
			reasoning = $5,
			savvy = $6,
			flair = $7
		WHERE id = $1`,
		characterID,
		abilities.Brutality, abilities.Grit, abilities.Quickness,
		abilities.Reasoning, abilities.Savvy, abilities.Flair,
	)
	if err != nil {
		return fmt.Errorf("saving abilities for character %d: %w", characterID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCharacterNotFound
	}
	return nil
}

// LoadWeaponPresets fetches all weapon preset rows for characterID and assembles a LoadoutSet.
// Returns a LoadoutSet with 2 empty presets when no rows exist.
//
// Precondition: characterID must be >= 0; reg must not be nil.
// Postcondition: Returns a non-nil *inventory.LoadoutSet and nil error on success.
// Weapon definitions found in reg are re-hydrated into the preset slots; unknown IDs are skipped.
func (r *CharacterRepository) LoadWeaponPresets(ctx context.Context, characterID int64, reg *inventory.Registry) (*inventory.LoadoutSet, error) {
	rows, err := r.db.Query(ctx, `
		SELECT preset_index, slot, item_def_id, ammo_count
		FROM character_weapon_presets
		WHERE character_id = $1
		ORDER BY preset_index, slot`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("loading weapon presets for character %d: %w", characterID, err)
	}
	defer rows.Close()

	ls := inventory.NewLoadoutSet()

	for rows.Next() {
		var presetIdx int
		var slot, itemDefID string
		var ammoCount int
		if err := rows.Scan(&presetIdx, &slot, &itemDefID, &ammoCount); err != nil {
			return nil, fmt.Errorf("scanning weapon preset row: %w", err)
		}
		// Grow Presets slice if needed (class features may add more presets).
		for len(ls.Presets) <= presetIdx {
			ls.Presets = append(ls.Presets, inventory.NewWeaponPreset())
		}
		// Look up the weapon definition from the registry and re-hydrate the preset slot.
		// Unknown item IDs (e.g. items removed from the registry) are silently skipped.
		def := reg.Weapon(itemDefID)
		if def == nil {
			continue
		}
		preset := ls.Presets[presetIdx]
		switch slot {
		case "main_hand":
			if equipErr := preset.EquipMainHand(def); equipErr != nil {
				return nil, fmt.Errorf("rehydrating main_hand preset %d for character %d: %w", presetIdx, characterID, equipErr)
			}
		case "off_hand":
			if equipErr := preset.EquipOffHand(def); equipErr != nil {
				return nil, fmt.Errorf("rehydrating off_hand preset %d for character %d: %w", presetIdx, characterID, equipErr)
			}
		}
		_ = ammoCount // ammo count restoration is deferred to magazine hydration feature
	}
	return ls, rows.Err()
}

// SaveWeaponPresets replaces all weapon preset rows for characterID.
//
// Precondition: characterID must be > 0; ls must not be nil.
// Postcondition: DB rows reflect ls exactly; returns nil on success.
func (r *CharacterRepository) SaveWeaponPresets(ctx context.Context, characterID int64, ls *inventory.LoadoutSet) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM character_weapon_presets WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return fmt.Errorf("clearing weapon presets for character %d: %w", characterID, err)
	}

	for i, preset := range ls.Presets {
		if preset.MainHand != nil {
			ammo := 0
			if preset.MainHand.Magazine != nil {
				ammo = preset.MainHand.Magazine.Loaded
			}
			if _, err := r.db.Exec(ctx, `
				INSERT INTO character_weapon_presets
					(character_id, preset_index, slot, item_def_id, ammo_count)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (character_id, preset_index, slot)
					DO UPDATE SET item_def_id = EXCLUDED.item_def_id,
					              ammo_count  = EXCLUDED.ammo_count`,
				characterID, i, "main_hand", preset.MainHand.Def.ID, ammo,
			); err != nil {
				return fmt.Errorf("saving main_hand for character %d preset %d: %w", characterID, i, err)
			}
		}
		if preset.OffHand != nil {
			ammo := 0
			if preset.OffHand.Magazine != nil {
				ammo = preset.OffHand.Magazine.Loaded
			}
			if _, err := r.db.Exec(ctx, `
				INSERT INTO character_weapon_presets
					(character_id, preset_index, slot, item_def_id, ammo_count)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (character_id, preset_index, slot)
					DO UPDATE SET item_def_id = EXCLUDED.item_def_id,
					              ammo_count  = EXCLUDED.ammo_count`,
				characterID, i, "off_hand", preset.OffHand.Def.ID, ammo,
			); err != nil {
				return fmt.Errorf("saving off_hand for character %d preset %d: %w", characterID, i, err)
			}
		}
	}
	return nil
}

// LoadEquipment fetches all equipment rows for characterID.
// Returns an empty Equipment when no rows exist.
//
// Precondition: characterID must be >= 0.
// Postcondition: Returns a non-nil *inventory.Equipment and nil error on success.
func (r *CharacterRepository) LoadEquipment(ctx context.Context, characterID int64) (*inventory.Equipment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT slot, item_def_id
		FROM character_equipment
		WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("loading equipment for character %d: %w", characterID, err)
	}
	defer rows.Close()

	eq := inventory.NewEquipment()
	for rows.Next() {
		var slot, itemDefID string
		if err := rows.Scan(&slot, &itemDefID); err != nil {
			return nil, fmt.Errorf("scanning equipment row: %w", err)
		}
		item := &inventory.SlottedItem{ItemDefID: itemDefID, Name: itemDefID}
		// Determine slot type and populate the appropriate map.
		// Full name hydration is deferred to feature #4 (weapon and armor library);
		// until then, Name is set to ItemDefID as a placeholder.
		switch inventory.ArmorSlot(slot) {
		case inventory.SlotHead, inventory.SlotLeftArm, inventory.SlotRightArm,
			inventory.SlotTorso, inventory.SlotHands, inventory.SlotLeftLeg, inventory.SlotRightLeg, inventory.SlotFeet:
			eq.Armor[inventory.ArmorSlot(slot)] = item
		default:
			// Treat as accessory slot.
			eq.Accessories[inventory.AccessorySlot(slot)] = item
		}
	}
	return eq, rows.Err()
}

// SaveEquipment replaces all equipment rows for characterID.
//
// Precondition: characterID must be > 0; eq must not be nil.
// Postcondition: DB rows reflect eq exactly; returns nil on success.
func (r *CharacterRepository) SaveEquipment(ctx context.Context, characterID int64, eq *inventory.Equipment) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM character_equipment WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return fmt.Errorf("clearing equipment for character %d: %w", characterID, err)
	}

	for slot, item := range eq.Armor {
		if item == nil {
			continue
		}
		if _, err := r.db.Exec(ctx, `
			INSERT INTO character_equipment (character_id, slot, item_def_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (character_id, slot)
				DO UPDATE SET item_def_id = EXCLUDED.item_def_id`,
			characterID, string(slot), item.ItemDefID,
		); err != nil {
			return fmt.Errorf("saving armor slot %s for character %d: %w", slot, characterID, err)
		}
	}
	for slot, item := range eq.Accessories {
		if item == nil {
			continue
		}
		if _, err := r.db.Exec(ctx, `
			INSERT INTO character_equipment (character_id, slot, item_def_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (character_id, slot)
				DO UPDATE SET item_def_id = EXCLUDED.item_def_id`,
			characterID, string(slot), item.ItemDefID,
		); err != nil {
			return fmt.Errorf("saving accessory slot %s for character %d: %w", slot, characterID, err)
		}
	}
	return nil
}

// LoadInventory fetches all backpack items for characterID.
//
// Precondition: characterID must be >= 0.
// Postcondition: Returns nil slice and nil error when no rows exist.
func (r *CharacterRepository) LoadInventory(ctx context.Context, characterID int64) ([]inventory.InventoryItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT item_def_id, quantity
		FROM character_inventory
		WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("loading inventory for character %d: %w", characterID, err)
	}
	defer rows.Close()

	var items []inventory.InventoryItem
	for rows.Next() {
		var it inventory.InventoryItem
		if err := rows.Scan(&it.ItemDefID, &it.Quantity); err != nil {
			return nil, fmt.Errorf("scanning inventory row: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// SaveInventory replaces all backpack rows for characterID.
//
// Precondition: characterID must be > 0.
// Postcondition: DB rows reflect items exactly; returns nil on success.
func (r *CharacterRepository) SaveInventory(ctx context.Context, characterID int64, items []inventory.InventoryItem) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM character_inventory WHERE character_id = $1`, characterID); err != nil {
		return fmt.Errorf("clearing inventory for character %d: %w", characterID, err)
	}
	for _, it := range items {
		if _, err := r.db.Exec(ctx, `
			INSERT INTO character_inventory (character_id, item_def_id, quantity)
			VALUES ($1, $2, $3)
			ON CONFLICT (character_id, item_def_id) DO UPDATE SET quantity = EXCLUDED.quantity`,
			characterID, it.ItemDefID, it.Quantity,
		); err != nil {
			return fmt.Errorf("saving inventory item %q for character %d: %w", it.ItemDefID, characterID, err)
		}
	}
	return nil
}

// HasReceivedStartingInventory returns whether the character has already received their starting kit.
//
// Precondition: characterID must be > 0.
// Postcondition: Returns false and ErrCharacterNotFound if no row exists; false and nil error if the flag is unset.
func (r *CharacterRepository) HasReceivedStartingInventory(ctx context.Context, characterID int64) (bool, error) {
	var received bool
	err := r.db.QueryRow(ctx, `
		SELECT has_received_starting_inventory
		FROM characters WHERE id = $1`,
		characterID,
	).Scan(&received)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrCharacterNotFound
		}
		return false, fmt.Errorf("checking starting inventory flag for character %d: %w", characterID, err)
	}
	return received, nil
}

// MarkStartingInventoryGranted sets has_received_starting_inventory = true for characterID.
//
// Precondition: characterID must be > 0.
// Postcondition: Flag is set; subsequent HasReceivedStartingInventory calls return true. Returns ErrCharacterNotFound if no row was updated.
func (r *CharacterRepository) MarkStartingInventoryGranted(ctx context.Context, characterID int64) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE characters SET has_received_starting_inventory = TRUE WHERE id = $1`,
		characterID,
	)
	if err != nil {
		return fmt.Errorf("marking starting inventory granted for character %d: %w", characterID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCharacterNotFound
	}
	return nil
}

// SaveProgress persists level, experience, max_hp and pending ability boost count
// for a character, delegating to CharacterProgressRepository.
//
// Precondition: id > 0; level >= 1; experience >= 0; maxHP >= 1; pendingBoosts >= 0.
// Postcondition: characters row and character_pending_boosts row are updated atomically.
func (r *CharacterRepository) SaveProgress(ctx context.Context, id int64, level, experience, maxHP, pendingBoosts int) error {
	return NewCharacterProgressRepository(r.db).SaveProgress(ctx, id, level, experience, maxHP, pendingBoosts)
}

// SaveCurrency persists the player's current currency (rounds) to the characters table.
//
// Precondition: id > 0; currency >= 0.
// Postcondition: characters.currency column is updated.
func (r *CharacterRepository) SaveCurrency(ctx context.Context, id int64, currency int) error {
	if id <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", id)
	}
	_, err := r.db.Exec(ctx, `UPDATE characters SET currency = $2 WHERE id = $1`, id, currency)
	return err
}

// LoadCurrency returns the stored currency value for the given character.
//
// Precondition: id > 0.
// Postcondition: returns (currency, nil) on success.
func (r *CharacterRepository) LoadCurrency(ctx context.Context, id int64) (int, error) {
	if id <= 0 {
		return 0, fmt.Errorf("characterID must be > 0, got %d", id)
	}
	var currency int
	err := r.db.QueryRow(ctx, `SELECT COALESCE(currency, 0) FROM characters WHERE id = $1`, id).Scan(&currency)
	return currency, err
}

// SaveHeroPoints persists the player's current hero point count.
//
// Precondition: id > 0; heroPoints >= 0.
// Postcondition: characters.hero_points column is updated.
func (r *CharacterRepository) SaveHeroPoints(ctx context.Context, id int64, heroPoints int) error {
	if id <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", id)
	}
	_, err := r.db.Exec(ctx, `UPDATE characters SET hero_points = $2 WHERE id = $1`, id, heroPoints)
	return err
}

// LoadHeroPoints returns the stored hero point count for the given character.
// Returns 0 for characters without a persisted count.
//
// Precondition: id > 0.
// Postcondition: returns (heroPoints, nil) on success.
func (r *CharacterRepository) LoadHeroPoints(ctx context.Context, id int64) (int, error) {
	if id <= 0 {
		return 0, fmt.Errorf("characterID must be > 0, got %d", id)
	}
	var heroPoints int
	err := r.db.QueryRow(ctx, `SELECT COALESCE(hero_points, 0) FROM characters WHERE id = $1`, id).Scan(&heroPoints)
	return heroPoints, err
}

// SaveJobs persists the player's job map and active job ID to the characters table.
// jobs may be nil or empty (stored as SQL NULL).
//
// Precondition: id > 0.
// Postcondition: characters.jobs and characters.active_job_id are updated.
func (r *CharacterRepository) SaveJobs(ctx context.Context, id int64, jobs map[string]int, activeJobID string) error {
	if id <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", id)
	}
	var jobsJSON *string
	if len(jobs) > 0 {
		b, err := json.Marshal(jobs)
		if err != nil {
			return fmt.Errorf("SaveJobs: marshalling jobs for character %d: %w", id, err)
		}
		s := string(b)
		jobsJSON = &s
	}
	_, err := r.db.Exec(ctx,
		`UPDATE characters SET jobs = $2, active_job_id = $3 WHERE id = $1`,
		id, jobsJSON, activeJobID,
	)
	if err != nil {
		return fmt.Errorf("SaveJobs: %w", err)
	}
	return nil
}

// LoadJobs returns the stored jobs map and active job ID for the given character.
// Returns an empty map and empty string when no jobs are stored.
//
// Precondition: id > 0.
// Postcondition: returns (jobs, activeJobID, nil) on success; jobs is never nil.
func (r *CharacterRepository) LoadJobs(ctx context.Context, id int64) (map[string]int, string, error) {
	if id <= 0 {
		return nil, "", fmt.Errorf("characterID must be > 0, got %d", id)
	}
	var jobsJSON *string
	var activeJobID string
	err := r.db.QueryRow(ctx,
		`SELECT jobs, COALESCE(active_job_id, '') FROM characters WHERE id = $1`,
		id,
	).Scan(&jobsJSON, &activeJobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return map[string]int{}, "", ErrCharacterNotFound
		}
		return nil, "", fmt.Errorf("LoadJobs: %w", err)
	}
	jobs := map[string]int{}
	if jobsJSON != nil && *jobsJSON != "" {
		if err := json.Unmarshal([]byte(*jobsJSON), &jobs); err != nil {
			return nil, "", fmt.Errorf("LoadJobs: unmarshalling jobs for character %d: %w", id, err)
		}
	}
	return jobs, activeJobID, nil
}

// InstanceChargeState holds persisted charge state for one item instance.
type InstanceChargeState struct {
	ChargesRemaining int
	Expended         bool
}

// SaveInstanceCharges upserts charges_remaining and expended for a backpack item instance.
// Requires itemDefID to satisfy the NOT NULL constraint on first insert.
//
// Precondition: characterID > 0; instanceID and itemDefID non-empty.
// Postcondition: DB row upserted; returns nil on success.
func (r *CharacterRepository) SaveInstanceCharges(ctx context.Context, characterID int64, instanceID, itemDefID string, charges int, expended bool) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO character_inventory_instances (instance_id, character_id, item_def_id, charges_remaining, expended)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (instance_id)
		DO UPDATE SET charges_remaining = EXCLUDED.charges_remaining, expended = EXCLUDED.expended`,
		instanceID, characterID, itemDefID, charges, expended,
	)
	return err
}

// LoadInstanceCharges returns a map of instanceID → InstanceChargeState for all instances
// belonging to characterID that have non-sentinel charge state (charges_remaining != -1).
//
// Precondition: characterID > 0.
// Postcondition: Returns nil map on error; empty map if no rows exist.
func (r *CharacterRepository) LoadInstanceCharges(ctx context.Context, characterID int64) (map[string]InstanceChargeState, error) {
	rows, err := r.db.Query(ctx, `
		SELECT instance_id, charges_remaining, expended
		FROM character_inventory_instances
		WHERE character_id = $1 AND charges_remaining != -1`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("loading instance charges for character %d: %w", characterID, err)
	}
	defer rows.Close()
	result := make(map[string]InstanceChargeState)
	for rows.Next() {
		var id string
		var s InstanceChargeState
		if err := rows.Scan(&id, &s.ChargesRemaining, &s.Expended); err != nil {
			return nil, fmt.Errorf("scanning instance charges: %w", err)
		}
		result[id] = s
	}
	return result, rows.Err()
}
