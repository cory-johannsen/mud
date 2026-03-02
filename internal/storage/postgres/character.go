package postgres

import (
	"context"
	"errors"
	"fmt"

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
			 max_hp, current_hp)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING id, account_id, name, region, class, team, level, experience, location,
		          brutality, quickness, grit, reasoning, savvy, flair,
		          max_hp, current_hp, created_at, updated_at`,
		c.AccountID, c.Name, c.Region, c.Class, c.Team, c.Level, c.Experience, c.Location,
		c.Abilities.Brutality, c.Abilities.Quickness, c.Abilities.Grit,
		c.Abilities.Reasoning, c.Abilities.Savvy, c.Abilities.Flair,
		c.MaxHP, c.CurrentHP,
	).Scan(
		&out.ID, &out.AccountID, &out.Name, &out.Region, &out.Class, &out.Team,
		&out.Level, &out.Experience, &out.Location,
		&out.Abilities.Brutality, &out.Abilities.Quickness, &out.Abilities.Grit,
		&out.Abilities.Reasoning, &out.Abilities.Savvy, &out.Abilities.Flair,
		&out.MaxHP, &out.CurrentHP, &out.CreatedAt, &out.UpdatedAt,
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
		       max_hp, current_hp, created_at, updated_at
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
			&c.MaxHP, &c.CurrentHP, &c.CreatedAt, &c.UpdatedAt,
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
		       max_hp, current_hp, created_at, updated_at
		FROM characters WHERE id = $1`,
		id,
	).Scan(
		&c.ID, &c.AccountID, &c.Name, &c.Region, &c.Class, &c.Team,
		&c.Level, &c.Experience, &c.Location,
		&c.Abilities.Brutality, &c.Abilities.Quickness, &c.Abilities.Grit,
		&c.Abilities.Reasoning, &c.Abilities.Savvy, &c.Abilities.Flair,
		&c.MaxHP, &c.CurrentHP, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCharacterNotFound
		}
		return nil, fmt.Errorf("querying character: %w", err)
	}
	return &c, nil
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

// LoadWeaponPresets fetches all weapon preset rows for characterID and assembles a LoadoutSet.
// Returns a LoadoutSet with 2 empty presets when no rows exist.
//
// Precondition: characterID must be >= 0.
// Postcondition: Returns a non-nil *inventory.LoadoutSet and nil error on success.
func (r *CharacterRepository) LoadWeaponPresets(ctx context.Context, characterID int64) (*inventory.LoadoutSet, error) {
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
		// NOTE: Full weapon hydration (populating MainHand/OffHand with a *WeaponDef) requires
		// the weapon registry, which is added in feature #4 (weapon and armor library).
		// Until then, the preset slot count is preserved but weapons are not reloaded.
		// Variables slot, itemDefID, and ammoCount are consumed to avoid compiler errors;
		// they will be used in feature #4.
		_, _, _ = slot, itemDefID, ammoCount
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
			inventory.SlotTorso, inventory.SlotLeftLeg, inventory.SlotRightLeg, inventory.SlotFeet:
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
