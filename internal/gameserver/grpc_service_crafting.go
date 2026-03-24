package gameserver

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/crafting"
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

// handleCraftList lists available recipes, optionally filtered by category. Each recipe
// line indicates whether quick crafting requires a higher rank than the player holds
// ("[downtime only]") and how many material types are missing ("[missing: N]").
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleCraftList(uid string, req *gamev1.CraftListRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if s.recipeReg == nil {
		return messageEvent("No recipes available."), nil
	}

	categoryFilter := strings.ToLower(strings.TrimSpace(req.GetCategory()))
	riggingRank := sess.Skills["rigging"]
	if riggingRank == "" {
		riggingRank = "untrained"
	}

	type entry struct {
		name    string
		line    string
		category string
	}

	var entries []entry
	for _, recipe := range s.recipeReg.All() {
		cat := strings.ToLower(recipe.Category)
		if categoryFilter != "" && cat != categoryFilter {
			continue
		}

		// Count missing material types.
		missingTypes := 0
		for _, rm := range recipe.Materials {
			if sess.Materials[rm.ID] < rm.Quantity {
				missingTypes++
			}
		}

		// Determine quick-craft eligibility by proficiency rank comparison.
		canQuickCraft := skillcheck.ProficiencyBonus(riggingRank) >= skillcheck.ProficiencyBonus(recipe.EffectiveMinRank())

		var flags []string
		if !canQuickCraft {
			flags = append(flags, "[downtime only]")
		}
		if missingTypes > 0 {
			flags = append(flags, fmt.Sprintf("[missing: %d]", missingTypes))
		}

		line := fmt.Sprintf("  %s - DC %d, %d days", recipe.Name, recipe.DC, recipe.DowntimeDays())
		if len(flags) > 0 {
			line += " " + strings.Join(flags, " ")
		}
		entries = append(entries, entry{name: recipe.Name, line: line, category: cat})
	}

	if len(entries) == 0 {
		if categoryFilter != "" {
			return messageEvent(fmt.Sprintf("No %s recipes available.", categoryFilter)), nil
		}
		return messageEvent("No recipes available."), nil
	}

	// Group by category, sort both categories and recipe names for deterministic output.
	byCategory := make(map[string][]entry)
	for _, e := range entries {
		byCategory[e.category] = append(byCategory[e.category], e)
	}
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
		catTitle := strings.Title(cat) //nolint:staticcheck
		sb.WriteString(fmt.Sprintf("--- %s ---\n", catTitle))
		recipeEntries := byCategory[cat]
		sort.Slice(recipeEntries, func(a, b int) bool {
			return recipeEntries[a].name < recipeEntries[b].name
		})
		for _, e := range recipeEntries {
			sb.WriteString(e.line + "\n")
		}
	}
	return messageEvent(strings.TrimRight(sb.String(), "\n")), nil
}

// handleCraft stages a recipe for crafting after validating material availability.
// Sets PendingCraftRecipeID on success so the player can confirm with handleCraftConfirm.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleCraft(uid string, req *gamev1.CraftRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if s.recipeReg == nil {
		return errorEvent("No recipes available."), nil
	}

	recipeID := strings.TrimSpace(req.GetRecipeId())
	recipe := s.findRecipe(recipeID)
	if recipe == nil {
		return errorEvent(fmt.Sprintf("Recipe not found: %s", recipeID)), nil
	}

	// Validate materials.
	var missing []string
	for _, rm := range recipe.Materials {
		have := sess.Materials[rm.ID]
		if have < rm.Quantity {
			name := rm.ID
			if s.materialReg != nil {
				if mat, matOK := s.materialReg.Material(rm.ID); matOK {
					name = mat.Name
				}
			}
			missing = append(missing, fmt.Sprintf("%s (need %d, have %d)", name, rm.Quantity, have))
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return errorEvent(fmt.Sprintf("Missing materials: %s", strings.Join(missing, ", "))), nil
	}

	sess.PendingCraftRecipeID = recipe.ID
	return messageEvent(fmt.Sprintf(
		"Ready to craft: %s (DC %d, %d days). Type 'craft confirm' to proceed.",
		recipe.Name, recipe.DC, recipe.DowntimeDays(),
	)), nil
}

// handleCraftConfirm executes a pending craft action, rolling the skill check and
// applying the PF2E quick-craft outcome rules via the CraftingEngine.
//
// Precondition: uid identifies an active player session.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleCraftConfirm(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if sess.PendingCraftRecipeID == "" {
		return errorEvent("No pending craft. Use 'craft <recipe>' first."), nil
	}
	if s.recipeReg == nil {
		sess.PendingCraftRecipeID = ""
		return errorEvent("Recipe no longer available."), nil
	}

	recipe, recipeOK := s.recipeReg.Recipe(sess.PendingCraftRecipeID)
	if !recipeOK {
		sess.PendingCraftRecipeID = ""
		return errorEvent("Recipe no longer available."), nil
	}

	// Re-validate materials (guard against race between craft and confirm).
	var missing []string
	for _, rm := range recipe.Materials {
		if sess.Materials[rm.ID] < rm.Quantity {
			missing = append(missing, rm.ID)
		}
	}
	if len(missing) > 0 {
		sess.PendingCraftRecipeID = ""
		sort.Strings(missing)
		return errorEvent(fmt.Sprintf("Missing materials: %s", strings.Join(missing, ", "))), nil
	}

	riggingRank := sess.Skills["rigging"]
	if riggingRank == "" {
		riggingRank = "untrained"
	}

	isQuickCraft := skillcheck.ProficiencyBonus(riggingRank) >= skillcheck.ProficiencyBonus(recipe.EffectiveMinRank())
	if !isQuickCraft {
		sess.PendingCraftRecipeID = ""
		return messageEvent(fmt.Sprintf(
			"Craft queued for downtime: %d days. (Use the downtime system to complete it.)",
			recipe.DowntimeDays(),
		)), nil
	}

	// Deduct materials via repo and session state.
	deductions := make(map[string]int, len(recipe.Materials))
	for _, rm := range recipe.Materials {
		deductions[rm.ID] = rm.Quantity
	}
	if s.materialRepo != nil {
		if err := s.materialRepo.DeductMany(context.Background(), sess.CharacterID, deductions); err != nil {
			sess.PendingCraftRecipeID = ""
			return errorEvent("Failed to deduct materials."), nil
		}
	}
	for matID, qty := range deductions {
		sess.Materials[matID] -= qty
		if sess.Materials[matID] <= 0 {
			delete(sess.Materials, matID)
		}
	}

	// Roll the crafting check. Use fallback roll=10 when dice is nil (test mode).
	var roll int
	if s.dice != nil {
		roll = s.dice.Src().Intn(20) + 1
	} else {
		roll = 10
	}
	abilityMod := (sess.Abilities.Savvy - 10) / 2
	profBonus := skillcheck.ProficiencyBonus(riggingRank)
	total := roll + abilityMod + profBonus
	checkOutcome := skillcheck.OutcomeFor(total, recipe.DC)
	craftOutcome := crafting.Outcome(checkOutcome)

	if s.craftEngine == nil {
		sess.PendingCraftRecipeID = ""
		return errorEvent("Crafting system not available."), nil
	}
	craftResult := s.craftEngine.ExecuteQuickCraft(recipe, sess.Materials, craftOutcome)
	sess.PendingCraftRecipeID = ""

	var sb strings.Builder
	switch craftOutcome {
	case crafting.CritSuccess:
		sb.WriteString(fmt.Sprintf("Critical success! You craft %d %s.", craftResult.OutputQuantity, recipe.Name))
	case crafting.Success:
		sb.WriteString(fmt.Sprintf("Success! You craft %d %s.", craftResult.OutputQuantity, recipe.Name))
	case crafting.Failure:
		sb.WriteString(fmt.Sprintf("Failure. You do not craft %s (some materials wasted).", recipe.Name))
	default:
		sb.WriteString(fmt.Sprintf("Critical failure! You do not craft %s (all materials lost).", recipe.Name))
	}

	// Add output items to backpack if crafting succeeded and backpack is available.
	if craftResult.OutputQuantity > 0 && sess.Backpack != nil && s.invRegistry != nil && recipe.OutputItemID != "" {
		_, _ = sess.Backpack.Add(recipe.OutputItemID, craftResult.OutputQuantity, s.invRegistry)
	}

	return messageEvent(sb.String()), nil
}

// findRecipe looks up a recipe by exact ID first, then by case-insensitive name match.
//
// Precondition: s.recipeReg must not be nil.
// Postcondition: Returns the matching *crafting.Recipe or nil if not found.
func (s *GameServiceServer) findRecipe(query string) *crafting.Recipe {
	if r, ok := s.recipeReg.Recipe(query); ok {
		return r
	}
	q := strings.ToLower(query)
	for _, r := range s.recipeReg.All() {
		if strings.ToLower(r.Name) == q {
			return r
		}
	}
	return nil
}
