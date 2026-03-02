package command

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// HandleChar returns a plain-text character sheet for the given session.
//
// Precondition: sess must be non-nil.
// Postcondition: Returns a non-empty string; never panics regardless of session state.
func HandleChar(sess *session.PlayerSession) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("=== %s ===\n", sess.CharName))
	sb.WriteString(fmt.Sprintf("Class: %s  Level: %d\n", sess.Class, sess.Level))
	sb.WriteString(fmt.Sprintf("HP: %d / %d\n\n", sess.CurrentHP, sess.MaxHP))

	sb.WriteString("--- Abilities ---\n")
	a := sess.Abilities
	sb.WriteString(fmt.Sprintf("BRT: %d  GRT: %d  QCK: %d\n", a.Brutality, a.Grit, a.Quickness))
	sb.WriteString(fmt.Sprintf("RSN: %d  SAV: %d  FLR: %d\n\n", a.Reasoning, a.Savvy, a.Flair))

	sb.WriteString("--- Weapons ---\n")
	if sess.LoadoutSet != nil {
		preset := sess.LoadoutSet.ActivePreset()
		if preset != nil {
			mainName := "(none)"
			offName := "(none)"
			if preset.MainHand != nil {
				mainName = preset.MainHand.Def.Name
			}
			if preset.OffHand != nil {
				offName = preset.OffHand.Def.Name
			}
			sb.WriteString(fmt.Sprintf("Main: %s\nOff:  %s\n\n", mainName, offName))
		} else {
			sb.WriteString("(no active loadout)\n\n")
		}
	} else {
		sb.WriteString("(no loadout)\n\n")
	}

	sb.WriteString("--- Currency ---\n")
	sb.WriteString(fmt.Sprintf("%s\n", inventory.FormatRounds(sess.Currency)))

	return sb.String()
}
