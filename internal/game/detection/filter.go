package detection

// FilterableNPC is the minimal NPC view shape FilterRoomView mutates. The
// gameserver's gamev1.NpcInfo satisfies this shape via field aliases; the
// filter is defined on a local interface so the detection package does not
// import the protobuf-generated package (and to ease testing).
type FilterableNPC interface {
	GetInstanceId() string
	GetName() string
}

// FilterDecision describes how a single NPC should be presented to a given
// observer after consulting the detection state.
type FilterDecision struct {
	// Drop, when true, means the NPC must be omitted from the recipient's
	// view entirely (Unnoticed).
	Drop bool
	// RedactName, when non-empty, replaces the NPC's display name in the
	// recipient's view (e.g. "???" for Undetected, "<silhouette>" for
	// Hidden). Empty means show the real name.
	RedactName string
	// HideCell, when true, instructs the renderer to omit the NPC's grid
	// position from the recipient's view (Undetected / Unnoticed /
	// Invisible-without-sound).
	HideCell bool
}

// DecideForNPC returns the FilterDecision the recipient observer should apply
// to a given NPC instance based on the detection state map. Pure function;
// safe for use in a broadcast loop.
//
// Mapping:
//
//	Observed   → no change
//	Concealed  → no change (concealment is a check-time effect, not a redact)
//	Hidden     → keep cell, redact name to "<silhouette>"
//	Undetected → hide cell, redact name to "???"
//	Unnoticed  → drop entirely
//	Invisible  → with sound cue: same as Hidden; otherwise same as Undetected
func DecideForNPC(observerUID, npcUID string, m *Map, soundCue bool) FilterDecision {
	switch m.Get(observerUID, npcUID) {
	case Observed, Concealed:
		return FilterDecision{}
	case Hidden:
		return FilterDecision{RedactName: "<silhouette>"}
	case Undetected:
		return FilterDecision{RedactName: "???", HideCell: true}
	case Unnoticed:
		return FilterDecision{Drop: true}
	case Invisible:
		if soundCue {
			return FilterDecision{RedactName: "<silhouette>"}
		}
		return FilterDecision{RedactName: "???", HideCell: true}
	}
	return FilterDecision{}
}
