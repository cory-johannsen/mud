package mentalstate

// Track identifies one of the four independent mental state tracks.
type Track int

const (
	TrackFear     Track = 0
	TrackRage     Track = 1
	TrackDespair  Track = 2
	TrackDelirium Track = 3
)

// Severity represents the intensity level of a mental state track.
type Severity int

const (
	SeverityNone   Severity = 0
	SeverityMild   Severity = 1
	SeverityMod    Severity = 2
	SeveritySevere Severity = 3
)

// StateChange is returned by mutating Manager methods to describe what changed.
type StateChange struct {
	Track          Track
	OldConditionID string
	NewConditionID string
	Message        string
}

// conditionIDs maps [track][severity] to the condition ID string.
var conditionIDs = [4][4]string{
	TrackFear:     {"", "fear_uneasy", "fear_panicked", "fear_psychotic"},
	TrackRage:     {"", "rage_irritated", "rage_enraged", "rage_berserker"},
	TrackDespair:  {"", "despair_discouraged", "despair_despairing", "despair_catatonic"},
	TrackDelirium: {"", "delirium_confused", "delirium_delirious", "delirium_hallucinatory"},
}

// severityNames maps [track][severity] to the display name.
var severityNames = [4][4]string{
	TrackFear:     {"", "Uneasy", "Panicked", "Psychotic"},
	TrackRage:     {"", "Irritated", "Enraged", "Berserker"},
	TrackDespair:  {"", "Discouraged", "Despairing", "Catatonic"},
	TrackDelirium: {"", "Confused", "Delirious", "Hallucinatory"},
}

// clearMessages maps track to the narrative message when cleared.
var clearMessages = [4]string{
	TrackFear:     "Your fear subsides.",
	TrackRage:     "Your rage fades.",
	TrackDespair:  "Your despair lifts.",
	TrackDelirium: "Your head clears.",
}

// escalateAfterRounds maps [track][severity] to rounds before escalation (0 = never).
var escalateAfterRounds = [4][4]int{
	TrackFear:     {0, 3, 5, 0},
	TrackRage:     {0, 4, 5, 0},
	TrackDespair:  {0, 5, 5, 0},
	TrackDelirium: {0, 4, 5, 0},
}

// autoRecoverAfterRounds maps [track][severity] to rounds before auto-recovery (0 = none).
var autoRecoverAfterRounds = [4][4]int{
	TrackFear:     {0, 3, 0, 0},
	TrackRage:     {0, 4, 0, 0},
	TrackDespair:  {0, 5, 0, 3},
	TrackDelirium: {0, 4, 0, 0},
}

// ConditionID returns the condition ID for a given track and severity.
func ConditionID(track Track, sev Severity) string {
	return conditionIDs[track][sev]
}
