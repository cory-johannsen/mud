// Package downtime provides pure activity definitions and engine logic for
// downtime activities available to players in safe rooms.
package downtime

import "strings"

// Activity defines a single downtime activity available to players.
//
// Precondition: ID, Name, and Alias are non-empty; DurationMinutes > 0 except
// for activities whose duration is computed at start time (craft, retrain).
// Postcondition: Activity values are read-only after package init.
type Activity struct {
	ID              string   // canonical ID
	Name            string   // display name
	Alias           string   // command alias (e.g., "earn", "craft")
	RequiredTags    []string // room must have ALL tags; always includes "safe"
	DurationMinutes int      // real-time duration in minutes; 0 = computed at start
	SkillID         string   // "" for Recalibrate/Retrain (no skill check)
	DefaultDC       int      // 0 = derived at resolution time
}

var activities = []Activity{
	{ID: "earn_creds", Name: "Earn Creds", Alias: "earn", RequiredTags: []string{"safe"}, DurationMinutes: 6, SkillID: "", DefaultDC: 0},
	{ID: "craft", Name: "Craft", Alias: "craft", RequiredTags: []string{"safe", "workshop"}, DurationMinutes: 0, SkillID: "rigging", DefaultDC: 0},
	{ID: "retrain", Name: "Retrain", Alias: "retrain", RequiredTags: []string{"safe"}, DurationMinutes: 0, SkillID: "", DefaultDC: 0},
	{ID: "fight_sickness", Name: "Fight the Sickness", Alias: "sickness", RequiredTags: []string{"safe", "clinic"}, DurationMinutes: 4, SkillID: "patch_job", DefaultDC: 0},
	{ID: "subsist", Name: "Subsist", Alias: "subsist", RequiredTags: []string{"safe"}, DurationMinutes: 2, SkillID: "", DefaultDC: 0},
	{ID: "forge_papers", Name: "Forge Papers", Alias: "forge", RequiredTags: []string{"safe"}, DurationMinutes: 6, SkillID: "hustle", DefaultDC: 15},
	{ID: "recalibrate", Name: "Recalibrate", Alias: "recalibrate", RequiredTags: []string{"safe"}, DurationMinutes: 1, SkillID: "", DefaultDC: 0},
	{ID: "patch_up", Name: "Patch Up", Alias: "patchup", RequiredTags: []string{"safe", "clinic"}, DurationMinutes: 1, SkillID: "patch_job", DefaultDC: 15},
	{ID: "flush_it", Name: "Flush It", Alias: "flushit", RequiredTags: []string{"safe", "clinic"}, DurationMinutes: 1, SkillID: "patch_job", DefaultDC: 0},
	{ID: "run_intel", Name: "Run Intel", Alias: "intel", RequiredTags: []string{"safe"}, DurationMinutes: 4, SkillID: "smooth_talk", DefaultDC: 15},
	{ID: "analyze_tech", Name: "Analyze Tech", Alias: "analyze", RequiredTags: []string{"safe"}, DurationMinutes: 2, SkillID: "tech_lore", DefaultDC: 0},
	{ID: "field_repair", Name: "Field Repair", Alias: "repair", RequiredTags: []string{"safe", "workshop"}, DurationMinutes: 2, SkillID: "rigging", DefaultDC: 0},
	{ID: "crack_code", Name: "Crack the Code", Alias: "decode", RequiredTags: []string{"safe"}, DurationMinutes: 3, SkillID: "", DefaultDC: 0},
	{ID: "run_cover", Name: "Run a Cover", Alias: "cover", RequiredTags: []string{"safe"}, DurationMinutes: 6, SkillID: "hustle", DefaultDC: 15},
	{ID: "apply_pressure", Name: "Apply Pressure", Alias: "pressure", RequiredTags: []string{"safe"}, DurationMinutes: 4, SkillID: "hard_look", DefaultDC: 0},
}

// AllActivities returns all 15 downtime activity definitions.
func AllActivities() []Activity { return activities }

// ActivityByAlias returns the activity with the given alias, if any.
//
// Precondition: alias is a lowercase command alias string.
// Postcondition: Returns (Activity, true) on match; (Activity{}, false) otherwise.
func ActivityByAlias(alias string) (Activity, bool) {
	for _, a := range activities {
		if a.Alias == alias {
			return a, true
		}
	}
	return Activity{}, false
}

// ActivityByID returns the activity with the given canonical ID, if any.
//
// Precondition: id is a canonical activity ID string.
// Postcondition: Returns (Activity, true) on match; (Activity{}, false) otherwise.
func ActivityByID(id string) (Activity, bool) {
	for _, a := range activities {
		if a.ID == id {
			return a, true
		}
	}
	return Activity{}, false
}

// TagsContain returns true if the comma-separated tags string contains the given tag.
func TagsContain(tags, tag string) bool {
	for _, t := range splitTags(tags) {
		if t == tag {
			return true
		}
	}
	return false
}

func splitTags(s string) []string {
	out := []string{}
	for _, part := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// AnalyzeTechTagsSatisfied returns true if the room has safe AND (workshop OR archive).
func AnalyzeTechTagsSatisfied(roomTags string) bool {
	return TagsContain(roomTags, "safe") &&
		(TagsContain(roomTags, "workshop") || TagsContain(roomTags, "archive"))
}
