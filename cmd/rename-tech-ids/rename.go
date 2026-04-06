package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// wordSepRE matches characters that should become word separators (underscores).
	wordSepRE = regexp.MustCompile(`[-_. ]+`)
	// nonAlphanumUnderscoreRE strips any remaining non-alphanumeric, non-underscore chars.
	nonAlphanumUnderscoreRE = regexp.MustCompile(`[^a-z0-9_]`)
	// multiUnderscoreRE collapses consecutive underscores.
	multiUnderscoreRE = regexp.MustCompile(`_+`)
)

// ToSnakeCase converts a human-readable name to a snake_case identifier.
// Hyphens, dots, underscores, and spaces are treated as word separators and
// become underscores; all other non-alphanumeric characters are removed;
// consecutive underscores are collapsed; leading/trailing underscores are trimmed.
func ToSnakeCase(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	withUnder := wordSepRE.ReplaceAllString(lower, "_")
	cleaned := nonAlphanumUnderscoreRE.ReplaceAllString(withUnder, "")
	collapsed := multiUnderscoreRE.ReplaceAllString(cleaned, "_")
	return strings.Trim(collapsed, "_")
}

var traditionSuffixes = []string{
	"_technical", "_neural", "_bio_synthetic", "_fanatic_doctrine",
}

// stripTraditionSuffix removes a known tradition suffix from id, if present.
func stripTraditionSuffix(id string) string {
	for _, s := range traditionSuffixes {
		if strings.HasSuffix(id, s) {
			return strings.TrimSuffix(id, s)
		}
	}
	return id
}

// pf2eKeywords is a deny-list of terms that indicate an un-localized PF2E name.
var pf2eKeywords = []string{
	"firebolt", "fireball", "magic missile", "telekinesis",
	"bestow curse", "mage hand", "shillelagh", "prestidigitation",
	"tongues", "scrying", "antimagic",
}

// IsPF2EFlagged returns true if name appears to be an unlocalised PF2E source name.
// REQ-TIR-PF2: derived new_id matches old_id minus tradition suffix → never localized.
//   This check only applies when old_id actually carries a tradition suffix.
// REQ-TIR-PF3: name contains a known PF2E keyword.
func IsPF2EFlagged(name, oldID string) bool {
	stripped := stripTraditionSuffix(oldID)
	if stripped != oldID && ToSnakeCase(name) == stripped {
		return true
	}
	lower := strings.ToLower(name)
	for _, kw := range pf2eKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// RenameEntry is one row in the rename map.
type RenameEntry struct {
	OldID     string `yaml:"old_id"`
	NewID     string `yaml:"new_id"`
	Name      string `yaml:"name"`
	File      string `yaml:"file"`
	Skip      bool   `yaml:"skip"`
	PF2EFlag  bool   `yaml:"pf2e_flag"`
	Collision bool   `yaml:"collision"`
}

// RenameMap is the top-level structure of tools/rename_map.yaml.
type RenameMap struct {
	Renames []RenameEntry `yaml:"renames"`
}

// techFileHeader holds just the id and name fields from a tech YAML file.
type techFileHeader struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

// BuildRenameMap scans all .yaml files under techDir, derives new IDs,
// detects collisions and PF2E flags, and returns the complete RenameMap
// sorted by old_id.
//
// Precondition: techDir is a valid, existing directory.
// Postcondition: all collision entries have Collision=true; all no-op entries have Skip=true.
func BuildRenameMap(techDir string) (*RenameMap, error) {
	var entries []RenameEntry

	err := filepath.WalkDir(techDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %q: %w", path, err)
		}
		var h techFileHeader
		if err := yaml.Unmarshal(data, &h); err != nil {
			return fmt.Errorf("parsing %q: %w", path, err)
		}
		if h.ID == "" || h.Name == "" {
			return fmt.Errorf("missing id or name in %q", path)
		}
		newID := ToSnakeCase(h.Name)
		entries = append(entries, RenameEntry{
			OldID:    h.ID,
			NewID:    newID,
			Name:     h.Name,
			File:     path,
			Skip:     h.ID == newID,
			PF2EFlag: IsPF2EFlagged(h.Name, h.ID),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Count occurrences of each new_id among non-skip entries to detect collisions.
	newIDCount := make(map[string]int)
	for _, e := range entries {
		if !e.Skip {
			newIDCount[e.NewID]++
		}
	}
	for i := range entries {
		if !entries[i].Skip && newIDCount[entries[i].NewID] > 1 {
			entries[i].Collision = true
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].OldID < entries[j].OldID
	})

	return &RenameMap{Renames: entries}, nil
}
