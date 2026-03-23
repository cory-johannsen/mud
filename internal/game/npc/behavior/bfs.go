package behavior

import (
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/world"
)

// BFSDistanceMap computes the BFS hop distance from originID to every reachable room
// in the provided room slice. Exits that target rooms not in the slice are ignored.
//
// Precondition: rooms must not be nil; originID must be the ID of a room in rooms.
// Postcondition: returns a map of roomID → hop count from originID (origin maps to 0);
// rooms unreachable from origin are absent from the map.
// Returns an error if originID is not found in rooms.
func BFSDistanceMap(rooms []*world.Room, originID string) (map[string]int, error) {
	roomByID := make(map[string]*world.Room, len(rooms))
	for _, r := range rooms {
		roomByID[r.ID] = r
	}
	if _, ok := roomByID[originID]; !ok {
		return nil, fmt.Errorf("behavior.BFSDistanceMap: origin %q not found in rooms", originID)
	}

	dist := make(map[string]int, len(rooms))
	dist[originID] = 0
	queue := []string{originID}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		room := roomByID[cur]
		for _, exit := range room.Exits {
			if _, exists := roomByID[exit.TargetRoom]; !exists {
				continue
			}
			if _, visited := dist[exit.TargetRoom]; !visited {
				dist[exit.TargetRoom] = dist[cur] + 1
				queue = append(queue, exit.TargetRoom)
			}
		}
	}
	return dist, nil
}
