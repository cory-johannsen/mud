// Package behavior implements NPC behavior evaluation helpers for the HTN system.
package behavior

import "math"

// PlayerSnapshot captures combat-relevant state of one player for threat assessment.
type PlayerSnapshot struct {
	Level     int
	CurrentHP int
	MaxHP     int
}

// ThreatScore computes the threat score for a group of players against an NPC of npcLevel.
//
// Formula: (playerAvgLevel - npcLevel) + (partySize-1)*2 - floor((1.0 - playerAvgHPPct) * 3)
//
// Precondition: npcLevel >= 1.
// Postcondition: returns 0 when players is empty; otherwise returns a signed int.
func ThreatScore(players []PlayerSnapshot, npcLevel int) int {
	if len(players) == 0 {
		return 0
	}
	partySize := len(players)
	sumLevel := 0
	sumHP := 0
	sumMaxHP := 0
	for _, p := range players {
		sumLevel += p.Level
		sumHP += p.CurrentHP
		sumMaxHP += p.MaxHP
	}
	avgLevel := float64(sumLevel) / float64(partySize)
	avgHPPct := 0.0
	if sumMaxHP > 0 {
		avgHPPct = float64(sumHP) / float64(sumMaxHP)
	}
	// Apply formula: (avgLevel - npcLevel) + (partySize-1)*2 - floor((1-avgHPPct)*3).
	// math.Floor truncates toward negative infinity per spec. REQ-NB-9.
	score := (avgLevel - float64(npcLevel)) +
		float64((partySize-1)*2) -
		math.Floor((1.0-avgHPPct)*3)
	// Convert to int by truncating; intermediate float precision is sufficient.
	return int(score)
}
