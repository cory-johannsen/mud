package combat

// InitiativeBonusForMargin maps a positive initiative margin to a combat bonus.
// Margin 1–5→+1, 6–10→+2, 11+→+3.
func InitiativeBonusForMargin(margin int) int {
	switch {
	case margin <= 5:
		return 1
	case margin <= 10:
		return 2
	default:
		return 3
	}
}

// RollInitiative rolls d20+DexMod for each combatant. If a player beats all NPCs,
// they receive an InitiativeBonus scaled by margin of victory.
func RollInitiative(combatants []*Combatant, src Source) {
	for _, c := range combatants {
		roll := src.Intn(20) + 1
		c.Initiative = roll + c.DexMod
	}
	var player *Combatant
	highestNPC := -1 << 30
	for _, c := range combatants {
		if c.Kind == KindPlayer {
			player = c
		} else if c.Initiative > highestNPC {
			highestNPC = c.Initiative
		}
	}
	if player == nil {
		return
	}
	if margin := player.Initiative - highestNPC; margin > 0 {
		player.InitiativeBonus = InitiativeBonusForMargin(margin)
	}
}
