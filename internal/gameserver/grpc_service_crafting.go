package gameserver

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/skillcheck"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// handleMaterials lists the player's crafting material inventory, optionally filtered
// by category. Materials are grouped by category and sorted alphabetically within each
// group.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleMaterials(uid string, req *gamev1.MaterialsRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}

	categoryFilter := strings.ToLower(strings.TrimSpace(req.GetCategory()))

	if len(sess.Materials) == 0 {
		return messageEvent("You have no materials."), nil
	}

	// Group materials by category.
	type entry struct {
		name     string
		quantity int
	}
	byCategory := make(map[string][]entry)

	// Sort material IDs for deterministic output.
	matIDs := make([]string, 0, len(sess.Materials))
	for id := range sess.Materials {
		matIDs = append(matIDs, id)
	}
	sort.Strings(matIDs)

	for _, id := range matIDs {
		qty := sess.Materials[id]
		if qty <= 0 {
			continue
		}
		name := id
		category := "uncategorized"
		if s.materialReg != nil {
			if mat, matOK := s.materialReg.Material(id); matOK {
				name = mat.Name
				if mat.Category != "" {
					category = mat.Category
				}
			}
		}
		if categoryFilter != "" && strings.ToLower(category) != categoryFilter {
			continue
		}
		byCategory[category] = append(byCategory[category], entry{name: name, quantity: qty})
	}

	if len(byCategory) == 0 {
		if categoryFilter != "" {
			return messageEvent(fmt.Sprintf("You have no %s materials.", categoryFilter)), nil
		}
		return messageEvent("You have no materials."), nil
	}

	// Sort categories alphabetically.
	categories := make([]string, 0, len(byCategory))
	for cat := range byCategory {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	var sb strings.Builder
	for i, cat := range categories {
		if i > 0 {
			sb.WriteString("\n")
		}
		catTitle := strings.Title(cat) //nolint:staticcheck // simple display title
		sb.WriteString(fmt.Sprintf("--- %s ---\n", catTitle))
		for _, e := range byCategory[cat] {
			sb.WriteString(fmt.Sprintf("  %s: %d\n", e.name, e.quantity))
		}
	}

	return messageEvent(strings.TrimRight(sb.String(), "\n")), nil
}

// handleScavenge attempts a scavenge action in the current room's zone.
// One attempt is allowed per room visit (REQ-CRAFT-11). On success, materials are
// drawn from the zone's weighted pool and added to the player's inventory.
//
// Precondition: uid identifies an active player session.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleScavenge(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}

	// REQ-CRAFT-11: one attempt per room visit.
	if sess.ScavengeExhaustedRoomID == sess.RoomID {
		return errorEvent("You've already scavenged this area."), nil
	}
	sess.ScavengeExhaustedRoomID = sess.RoomID

	room, roomOK := s.world.GetRoom(sess.RoomID)
	if !roomOK || room.ZoneID == "" {
		return messageEvent("There's nothing useful to scavenge here."), nil
	}

	zone, zoneOK := s.world.GetZone(room.ZoneID)
	if !zoneOK || zone.MaterialPool == nil {
		// REQ-CRAFT-9: zone has no pool, yield nothing.
		return messageEvent("There's nothing useful to scavenge here."), nil
	}

	pool := zone.MaterialPool

	if len(pool.Drops) == 0 {
		return messageEvent("There's nothing useful to scavenge here."), nil
	}

	scavRank, hasRank := sess.Skills["scavenging"]
	if !hasRank || scavRank == "" {
		scavRank = "untrained"
	}

	var roll int
	if s.dice != nil {
		roll = s.dice.Src().Intn(20) + 1
	} else {
		roll = 10 // deterministic fallback for nil-dice tests
	}

	abilityMod := (sess.Abilities.Savvy - 10) / 2
	result := skillcheck.Resolve(roll, abilityMod, scavRank, pool.DC, skillcheck.TriggerDef{})

	var count int
	switch result.Outcome {
	case skillcheck.CritSuccess:
		count = 3
	case skillcheck.Success:
		// 1-2 items
		if s.dice != nil {
			count = s.dice.Src().Intn(2) + 1
		} else {
			count = 1
		}
	default:
		return messageEvent("You scavenge the area but find nothing useful."), nil
	}

	gained := s.drawFromPool(pool.Drops, count)
	for matID, qty := range gained {
		if sess.Materials == nil {
			sess.Materials = make(map[string]int)
		}
		sess.Materials[matID] += qty
		if s.materialRepo != nil {
			_ = s.materialRepo.Add(context.Background(), sess.CharacterID, matID, qty)
		}
	}

	return messageEvent(s.formatScavengeResult(gained)), nil
}

// drawFromPool selects count items from a weighted drop pool (sampling with replacement).
//
// Precondition: count >= 0; drops must have positive Weight values.
// Postcondition: Returns a map from material ID to quantity gained; never nil.
func (s *GameServiceServer) drawFromPool(drops []world.MaterialPoolDrop, count int) map[string]int {
	result := make(map[string]int)
	if len(drops) == 0 || count <= 0 {
		return result
	}

	totalWeight := 0
	for _, d := range drops {
		totalWeight += d.Weight
	}
	if totalWeight <= 0 {
		return result
	}

	for i := 0; i < count; i++ {
		var r int
		if s.dice != nil {
			r = s.dice.Src().Intn(totalWeight)
		} else {
			r = 0 // deterministic fallback for nil-dice tests
		}
		cumulative := 0
		for _, d := range drops {
			cumulative += d.Weight
			if r < cumulative {
				result[d.ID]++
				break
			}
		}
	}

	return result
}

// formatScavengeResult builds a human-readable summary of gained materials.
//
// Precondition: gained may be empty.
// Postcondition: Returns a non-empty string.
func (s *GameServiceServer) formatScavengeResult(gained map[string]int) string {
	if len(gained) == 0 {
		return "You scavenge the area but find nothing useful."
	}

	// Sort gained keys for deterministic output.
	matIDs := make([]string, 0, len(gained))
	for id := range gained {
		matIDs = append(matIDs, id)
	}
	sort.Strings(matIDs)

	parts := []string{"You find:"}
	for _, matID := range matIDs {
		qty := gained[matID]
		name := matID
		if s.materialReg != nil {
			if mat, ok := s.materialReg.Material(matID); ok {
				name = mat.Name
			}
		}
		parts = append(parts, fmt.Sprintf("  %s x%d", name, qty))
	}

	return strings.Join(parts, "\n")
}
