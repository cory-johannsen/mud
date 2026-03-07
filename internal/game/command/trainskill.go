package command

import (
	"fmt"
	"strings"
)

// ValidSkillIDs is the complete list of skill IDs accepted by HandleTrainSkill.
// These match the skill IDs defined in content/skills.yaml.
var ValidSkillIDs = []string{
	"parkour", "ghosting", "grift", "muscle",
	"tech_lore", "rigging", "conspiracy", "factions", "intel",
	"patch_job", "wasteland", "gang_codes", "scavenging",
	"hustle", "smooth_talk", "hard_look", "rep",
}

var validSkillSet = func() map[string]bool {
	m := make(map[string]bool, len(ValidSkillIDs))
	for _, s := range ValidSkillIDs {
		m[s] = true
	}
	return m
}()

// HandleTrainSkill validates a trainskill command argument.
//
// Precondition: args contains the raw arguments after the command name.
// Postcondition: returns the normalized skill ID on success;
// returns an error with usage hint if args is empty;
// returns an error with "unknown skill" if the skill ID is invalid.
func HandleTrainSkill(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: trainskill <skill>  (e.g. trainskill parkour)")
	}
	skillID := strings.ToLower(strings.TrimSpace(args[0]))
	if !validSkillSet[skillID] {
		return "", fmt.Errorf("unknown skill %q; valid skills: %s", skillID, strings.Join(ValidSkillIDs, ", "))
	}
	return skillID, nil
}
