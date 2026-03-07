package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterSkillsRepository persists per-character skill proficiency ranks.
type CharacterSkillsRepository struct {
	db *pgxpool.Pool
}

// NewCharacterSkillsRepository creates a repository backed by the given pool.
//
// Precondition: db must be a valid, open connection pool.
func NewCharacterSkillsRepository(db *pgxpool.Pool) *CharacterSkillsRepository {
	return &CharacterSkillsRepository{db: db}
}

// HasSkills reports whether the character has any rows in character_skills.
//
// Precondition: characterID > 0.
// Postcondition: Returns true if at least one skill row exists.
func (r *CharacterSkillsRepository) HasSkills(ctx context.Context, characterID int64) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM character_skills WHERE character_id = $1`, characterID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("HasSkills: %w", err)
	}
	return count > 0, nil
}

// GetAll returns all skill proficiency ranks for a character.
//
// Precondition: characterID > 0.
// Postcondition: Returns a map of skill_id → proficiency (may be empty).
func (r *CharacterSkillsRepository) GetAll(ctx context.Context, characterID int64) (map[string]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT skill_id, proficiency FROM character_skills WHERE character_id = $1`, characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetAll skills: %w", err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var id, prof string
		if err := rows.Scan(&id, &prof); err != nil {
			return nil, fmt.Errorf("scanning skill row: %w", err)
		}
		out[id] = prof
	}
	return out, rows.Err()
}

// SetAll writes the complete skill map for a character, replacing any existing rows.
//
// Precondition: characterID > 0; skills must not be nil.
// Postcondition: character_skills rows match skills exactly.
func (r *CharacterSkillsRepository) SetAll(ctx context.Context, characterID int64, skills map[string]string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM character_skills WHERE character_id = $1`, characterID,
	); err != nil {
		return fmt.Errorf("deleting old skills: %w", err)
	}

	for skillID, prof := range skills {
		if _, err := tx.Exec(ctx,
			`INSERT INTO character_skills (character_id, skill_id, proficiency) VALUES ($1, $2, $3)`,
			characterID, skillID, prof,
		); err != nil {
			return fmt.Errorf("inserting skill %s: %w", skillID, err)
		}
	}
	return tx.Commit(ctx)
}

// UpgradeSkill upserts a single skill rank for a character.
//
// Precondition: characterID > 0; skillID and newRank must be non-empty.
// Postcondition: character_skills row for (characterID, skillID) has proficiency = newRank.
func (r *CharacterSkillsRepository) UpgradeSkill(ctx context.Context, characterID int64, skillID, newRank string) error {
	if characterID <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", characterID)
	}
	if skillID == "" || newRank == "" {
		return fmt.Errorf("skillID and newRank must be non-empty")
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO character_skills (character_id, skill_id, proficiency)
		VALUES ($1, $2, $3)
		ON CONFLICT (character_id, skill_id) DO UPDATE SET proficiency = EXCLUDED.proficiency
	`, characterID, skillID, newRank)
	if err != nil {
		return fmt.Errorf("UpgradeSkill: %w", err)
	}
	return nil
}
