package quest

import (
	"context"
	"time"
)

// CompletionResult carries structured data about a quest that just completed,
// for use by callers that need to build UI events (e.g. QuestCompleteEvent).
type CompletionResult struct {
	QuestID       string
	Title         string
	XPReward      int
	CreditsReward int
	ItemRewards   []string // human-readable strings, e.g. "Combat Stims x2"
}

// ActiveQuest is the in-session state of a quest the player is currently on.
type ActiveQuest struct {
	QuestID           string
	ObjectiveProgress map[string]int // objective ID → count completed so far
}

// QuestRecord is the persisted row returned from LoadQuests.
type QuestRecord struct {
	CharacterID int64
	QuestID     string
	Status      string // "active" | "completed" | "abandoned"
	Progress    map[string]int
	CompletedAt *time.Time
}

// QuestRepository is the storage interface for quest persistence.
//
// Precondition: characterID > 0; questID and objectiveID must be non-empty.
type QuestRepository interface {
	// SaveQuestStatus upserts the character_quests row.
	SaveQuestStatus(ctx context.Context, characterID int64, questID, status string, completedAt *time.Time) error
	// SaveObjectiveProgress upserts the character_quest_progress row.
	SaveObjectiveProgress(ctx context.Context, characterID int64, questID, objectiveID string, progress int) error
	// LoadQuests returns all quest rows for a character (join of both tables).
	LoadQuests(ctx context.Context, characterID int64) ([]QuestRecord, error)
}
