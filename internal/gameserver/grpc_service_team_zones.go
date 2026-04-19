package gameserver

// teamEnemyZones maps team ID to the zone ID that is enemy territory for that team.
// Team Gun's home is Vantucky; their enemy zone is Rustbucket Ridge.
// Team Machete's home is Rustbucket Ridge; their enemy zone is Vantucky.
var teamEnemyZones = map[string]string{
	"gun":     "rustbucket_ridge",
	"machete": "vantucky",
}

// teamHomeRooms maps team ID to the start room ID used when redirecting a player
// who has somehow ended up in enemy territory (e.g. teleport, persistence rollback).
var teamHomeRooms = map[string]string{
	"gun":     "vantucky_the_compound",
	"machete": "rust_scrap_office",
}

// isEnemyZone returns true if zoneID is enemy territory for the given team.
//
// Precondition: teamID and zoneID must be non-empty for meaningful results.
// Postcondition: Returns false for unknown team IDs or non-enemy zones.
func isEnemyZone(teamID, zoneID string) bool {
	enemyZone, ok := teamEnemyZones[teamID]
	if !ok {
		return false
	}
	return enemyZone == zoneID
}
