package combat

import "fmt"

// InitiationReason identifies why combat was initiated.
type InitiationReason string

const (
	ReasonOnSight    InitiationReason = "on_sight"
	ReasonTerritory  InitiationReason = "territory"
	ReasonProvoked   InitiationReason = "provoked"
	ReasonCallForHelp InitiationReason = "call_for_help"
	ReasonWanted     InitiationReason = "wanted"
	ReasonProtecting InitiationReason = "protecting"
)

// FormatPlayerInitiationMsg returns "You attack [targetName]." COMBATMSG-5.
func FormatPlayerInitiationMsg(targetName string) string {
	return fmt.Sprintf("You attack %s.", targetName)
}

// FormatNPCInitiationMsg returns "[npcName] attacks you — [reason phrase]." COMBATMSG-6.
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
