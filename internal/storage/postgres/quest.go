package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cory-johannsen/mud/internal/game/quest"
)

// QuestRepository implements quest.QuestRepository backed by PostgreSQL.
type QuestRepository struct {
	db *pgxpool.Pool
}

// NewQuestRepository creates a new QuestRepository.
//
// Precondition: db must be non-nil.
func NewQuestRepository(db *pgxpool.Pool) *QuestRepository {
	return &QuestRepository{db: db}
}

// SaveQuestStatus upserts the character_quests row for (characterID, questID).
func (r *QuestRepository) SaveQuestStatus(ctx context.Context, characterID int64, questID, status string, completedAt *time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO character_quests (character_id, quest_id, status, completed_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (character_id, quest_id) DO UPDATE
		  SET status = EXCLUDED.status, completed_at = EXCLUDED.completed_at
	`, characterID, questID, status, completedAt)
	if err != nil {
		return fmt.Errorf("SaveQuestStatus: %w", err)
	}
	return nil
}

// SaveObjectiveProgress upserts the character_quest_progress row.
func (r *QuestRepository) SaveObjectiveProgress(ctx context.Context, characterID int64, questID, objectiveID string, progress int) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO character_quest_progress (character_id, quest_id, objective_id, progress)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (character_id, quest_id, objective_id) DO UPDATE
		  SET progress = EXCLUDED.progress
	`, characterID, questID, objectiveID, progress)
	if err != nil {
		return fmt.Errorf("SaveObjectiveProgress: %w", err)
	}
	return nil
}

// LoadQuests returns all quest records for a character by joining both tables.
// Objectives with no progress rows produce an empty Progress map.
func (r *QuestRepository) LoadQuests(ctx context.Context, characterID int64) ([]quest.QuestRecord, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cq.quest_id, cq.status, cq.completed_at,
		       coalesce(cqp.objective_id, '') AS objective_id,
		       coalesce(cqp.progress, 0)      AS progress
		FROM character_quests cq
		LEFT JOIN character_quest_progress cqp
		  ON cq.character_id = cqp.character_id AND cq.quest_id = cqp.quest_id
		WHERE cq.character_id = $1
		ORDER BY cq.quest_id, cqp.objective_id
	`, characterID)
	if err != nil {
		return nil, fmt.Errorf("LoadQuests query: %w", err)
	}
	defer rows.Close()

	byID := make(map[string]*quest.QuestRecord)
	var order []string
	for rows.Next() {
		var questID, status, objID string
		var completedAt *time.Time
		var prog int
		if err := rows.Scan(&questID, &status, &completedAt, &objID, &prog); err != nil {
			return nil, fmt.Errorf("LoadQuests scan: %w", err)
		}
		rec, exists := byID[questID]
		if !exists {
			rec = &quest.QuestRecord{
				CharacterID: characterID,
				QuestID:     questID,
				Status:      status,
				CompletedAt: completedAt,
				Progress:    make(map[string]int),
			}
			byID[questID] = rec
			order = append(order, questID)
		}
		if objID != "" {
			rec.Progress[objID] = prog
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("LoadQuests rows: %w", err)
	}
	result := make([]quest.QuestRecord, 0, len(order))
	for _, id := range order {
		result = append(result, *byID[id])
	}
	return result, nil
}

// Ensure QuestRepository satisfies the quest.QuestRepository interface at compile time.
var _ quest.QuestRepository = (*QuestRepository)(nil)
