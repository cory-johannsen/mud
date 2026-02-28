package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cory-johannsen/mud/internal/game/character"
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
