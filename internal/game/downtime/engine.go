package downtime

// CanStart validates preconditions before starting an activity.
// Returns an error string if not allowed; empty string if OK.
//
// Precondition: activityAlias is a command alias string; roomTags is a comma-separated
// tag string from room.Properties["tags"]; currentBusy indicates an active activity.
// Postcondition: Returns "" if the activity may start; a human-readable error otherwise.
// StartNextFn is the type of the auto-start function called after activity completion.
// Implementations live in grpc_service.go (has access to session, world, DB, queue repo).
// The function pops from the queue, validates, and starts the next eligible activity.
// It calls itself recursively if the popped entry is invalid (REQ-DTQ-10).
type StartNextFn func(uid string)

func CanStart(activityAlias string, roomTags string, currentBusy bool) string {
	if currentBusy {
		return "You already have an active downtime activity."
	}
	if !TagsContain(roomTags, "safe") {
		return "You must be in a Safe room to begin a downtime activity."
	}
	act, ok := ActivityByAlias(activityAlias)
	if !ok {
		return "Unknown downtime activity."
	}
	// Special case: analyze_tech accepts workshop OR archive
	if act.ID == "analyze_tech" {
		if !AnalyzeTechTagsSatisfied(roomTags) {
			return "Analyze Tech requires a room with a workshop or archive."
		}
		return ""
	}
	// General case: check all required tags beyond "safe"
	for _, tag := range act.RequiredTags {
		if tag == "safe" {
			continue
		}
		if !TagsContain(roomTags, tag) {
			return "This room does not have the required facilities for that activity."
		}
	}
	return ""
}
