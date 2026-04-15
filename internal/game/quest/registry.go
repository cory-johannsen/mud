package quest

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// QuestRegistry is a map from quest ID to its definition.
type QuestRegistry map[string]*QuestDef

// LoadFromDir reads all *.yaml files in dir, parses each as a QuestDef,
// validates it, and returns the populated registry.
//
// Precondition: dir must be a readable directory path.
// Postcondition: All returned entries have passed Validate(); error is non-nil on any failure.
func LoadFromDir(dir string) (QuestRegistry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading quest dir %q: %w", dir, err)
	}
	reg := make(QuestRegistry)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading quest file %q: %w", path, err)
		}
		var def QuestDef
		if err := yaml.Unmarshal(data, &def); err != nil {
			return nil, fmt.Errorf("parsing quest file %q: %w", path, err)
		}
		if err := def.Validate(); err != nil {
			return nil, fmt.Errorf("invalid quest in %q: %w", path, err)
		}
		reg[def.ID] = &def
	}
	return reg, nil
}

// CrossValidate checks all quest definition references against live registries.
// npcIDs is the set of all known NPC template IDs; itemIDs the set of item def IDs;
// roomIDs the set of all room IDs in the world.
//
// Precondition: all maps must be non-nil (may be empty).
// Postcondition: Returns a fatal error if any reference is unresolvable.
func (r QuestRegistry) CrossValidate(npcIDs, itemIDs, roomIDs map[string]bool) error {
	for _, def := range r {
		// Skip NPC check for find_trainer quests — they have no giver NPC.
		if def.Type != "find_trainer" && !npcIDs[def.GiverNPCID] {
			return fmt.Errorf("quest %q: GiverNPCID %q not found in NPC registry", def.ID, def.GiverNPCID)
		}
		for _, prereq := range def.Prerequisites {
			if _, ok := r[prereq]; !ok {
				return fmt.Errorf("quest %q: prerequisite quest %q not found in QuestRegistry", def.ID, prereq)
			}
		}
		for _, obj := range def.Objectives {
			switch obj.Type {
			case "kill":
				if !npcIDs[obj.TargetID] {
					return fmt.Errorf("quest %q objective %q: kill TargetID %q not in NPC registry", def.ID, obj.ID, obj.TargetID)
				}
			case "fetch":
				if !itemIDs[obj.TargetID] {
					return fmt.Errorf("quest %q objective %q: fetch TargetID %q not in item registry", def.ID, obj.ID, obj.TargetID)
				}
			case "explore":
				if !roomIDs[obj.TargetID] {
					return fmt.Errorf("quest %q objective %q: explore TargetID %q not in world rooms", def.ID, obj.ID, obj.TargetID)
				}
			case "deliver":
				if !npcIDs[obj.TargetID] {
					return fmt.Errorf("quest %q objective %q: deliver TargetID %q not in NPC registry", def.ID, obj.ID, obj.TargetID)
				}
				if !itemIDs[obj.ItemID] {
					return fmt.Errorf("quest %q objective %q: deliver ItemID %q not in item registry", def.ID, obj.ID, obj.ItemID)
				}
			}
		}
	}
	return nil
}
