package combat

import "fmt"

// InitiationReason identifies why combat was initiated.
// Used to format the combat initiation message per COMBATMSG-3/4.
type InitiationReason string

const (
	// ReasonOnSight — NPC is unconditionally hostile (disposition=="hostile"). COMBATMSG-4a.
	ReasonOnSight InitiationReason = "on_sight"
	// ReasonTerritory — NPC attacked due to faction-based hostility. COMBATMSG-4b.
	ReasonTerritory InitiationReason = "territory"
	// ReasonProvoked — NPC retaliated after being struck first. COMBATMSG-4c.
	ReasonProvoked InitiationReason = "provoked"
	// ReasonCallForHelp — NPC joined combat via call_for_help mechanic. COMBATMSG-4d.
	ReasonCallForHelp InitiationReason = "call_for_help"
	// ReasonWanted — Guard NPC attacked due to player wanted level. COMBATMSG-4e.
	ReasonWanted InitiationReason = "wanted"
	// ReasonProtecting — NPC joined combat to defend a named ally. COMBATMSG-4f.
	ReasonProtecting InitiationReason = "protecting"
)

// FormatPlayerInitiationMsg returns the player-initiated combat message. COMBATMSG-5.
//
// Precondition: targetName must be non-empty.
// Postcondition: Returns "You attack [targetName]."
func FormatPlayerInitiationMsg(targetName string) string {
	return fmt.Sprintf("You attack %s.", targetName)
}

// FormatNPCInitiationMsg returns the NPC-initiated combat message. COMBATMSG-6.
//
// Precondition: npcName must be non-empty; reason must be a valid InitiationReason constant.
// Postcondition: Returns "[npcName] attacks you — [reason phrase]."
// When reason is ReasonProtecting and protectedNPCName is empty, falls back to call-for-help phrasing.
func FormatNPCInitiationMsg(npcName string, reason InitiationReason, protectedNPCName string) string {
	var phrase string
	switch reason {
	case ReasonOnSight:
		phrase = "attacked on sight"
	case ReasonTerritory:
		phrase = "defending its territory"
	case ReasonProvoked:
		phrase = "provoked by your attack"
	case ReasonCallForHelp:
		phrase = "responding to a call for help"
	case ReasonWanted:
		phrase = "alerted by your wanted status"
	case ReasonProtecting:
		if protectedNPCName != "" {
			phrase = fmt.Sprintf("protecting %s", protectedNPCName)
		} else {
			phrase = "responding to a call for help"
		}
	default:
		phrase = "attacked on sight"
	}
	return fmt.Sprintf("%s attacks you — %s.", npcName, phrase)
}
