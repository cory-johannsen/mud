package quest

import (
	"fmt"
	"time"
)

var validObjectiveTypes = map[string]bool{
	"kill": true, "fetch": true, "explore": true, "deliver": true, "use_zone_map": true,
}

type QuestRewardItem struct {
	ItemID   string `yaml:"item_id"`
	Quantity int    `yaml:"quantity"`
}

type QuestRewards struct {
	XP      int               `yaml:"xp"`
	Credits int               `yaml:"credits"`
	Items   []QuestRewardItem `yaml:"items"`
}

type QuestObjective struct {
	ID          string `yaml:"id"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	TargetID    string `yaml:"target_id"`
	ItemID      string `yaml:"item_id,omitempty"`
	Quantity    int    `yaml:"quantity"`
}

type QuestDef struct {
	ID            string           `yaml:"id"`
	Title         string           `yaml:"title"`
	Description   string           `yaml:"description"`
	Type          string           `yaml:"type,omitempty"`
	GiverNPCID    string           `yaml:"giver_npc_id"`
	Repeatable    bool             `yaml:"repeatable"`
	AutoComplete  bool             `yaml:"auto_complete,omitempty"`
	Cooldown      string           `yaml:"cooldown,omitempty"`
	Prerequisites []string         `yaml:"prerequisites,omitempty"`
	Objectives    []QuestObjective `yaml:"objectives"`
	Rewards       QuestRewards     `yaml:"rewards"`
}

func (d QuestDef) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("quest ID must not be empty")
	}
	if d.Title == "" {
		return fmt.Errorf("quest %q: Title must not be empty", d.ID)
	}
	// find_trainer quests have no NPC giver and no objectives — skip all checks.
	if d.Type == "find_trainer" {
		return nil
	}
	// onboarding quests have no NPC giver but DO have objectives.
	if d.Type != "onboarding" {
		if d.GiverNPCID == "" {
			return fmt.Errorf("quest %q: GiverNPCID must not be empty", d.ID)
		}
	}
	if len(d.Objectives) == 0 {
		return fmt.Errorf("quest %q: Objectives must not be empty", d.ID)
	}
	for _, obj := range d.Objectives {
		if obj.ID == "" {
			return fmt.Errorf("quest %q: objective ID must not be empty", d.ID)
		}
		if obj.Description == "" {
			return fmt.Errorf("quest %q objective %q: Description must not be empty", d.ID, obj.ID)
		}
		if obj.TargetID == "" {
			return fmt.Errorf("quest %q objective %q: TargetID must not be empty", d.ID, obj.ID)
		}
		if !validObjectiveTypes[obj.Type] {
			return fmt.Errorf("quest %q objective %q: invalid Type %q", d.ID, obj.ID, obj.Type)
		}
		if obj.Quantity < 1 {
			return fmt.Errorf("quest %q objective %q: Quantity must be >= 1", d.ID, obj.ID)
		}
		if obj.Type == "deliver" && obj.ItemID == "" {
			return fmt.Errorf("quest %q objective %q: deliver objective requires ItemID", d.ID, obj.ID)
		}
	}
	if !d.Repeatable && d.Cooldown != "" {
		return fmt.Errorf("quest %q: non-repeatable quest must not have Cooldown", d.ID)
	}
	if d.Cooldown != "" {
		if _, err := time.ParseDuration(d.Cooldown); err != nil {
			return fmt.Errorf("quest %q: invalid Cooldown %q: %w", d.ID, d.Cooldown, err)
		}
	}
	return nil
}
