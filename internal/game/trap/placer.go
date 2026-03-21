package trap

import (
	"math/rand"

	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/google/uuid"
)

// DefaultProbabilities maps danger level to (roomChance, coverChance) in [0,1].
var DefaultProbabilities = map[string][2]float64{
	"safe":        {0.0, 0.0},
	"sketchy":     {0.0, 0.15},
	"dangerous":   {0.35, 0.50},
	"all_out_war": {0.60, 0.75},
}

// selectFromPool picks one template ID from pool using weighted random selection.
// Returns "" if pool is empty.
func selectFromPool(pool []world.TrapPoolEntry, rng *rand.Rand) string {
	if len(pool) == 0 {
		return ""
	}
	total := 0
	for _, e := range pool {
		total += e.Weight
	}
	if total == 0 {
		return ""
	}
	r := rng.Intn(total)
	cumulative := 0
	for _, e := range pool {
		cumulative += e.Weight
		if r < cumulative {
			return e.Template
		}
	}
	return pool[len(pool)-1].Template
}

// PlaceTraps procedurally populates TrapManager with traps for zone.
// Static room/equipment traps (from zone YAML) are registered first and never overwritten (REQ-TR-9).
// Procedurally placed traps use one_shot or auto reset mode only (REQ-TR-10).
//
// Precondition: zone, templates, mgr, and rng must be non-nil.
// Precondition: defaultPool is the fallback pool from content/traps/defaults.yaml.
// Postcondition: All trap instances in the zone are registered in mgr, Armed=true.
func PlaceTraps(zone *world.Zone, templates map[string]*TrapTemplate, defaultPool []world.TrapPoolEntry, mgr *TrapManager, rng *rand.Rand) {
	// Determine effective pool for this zone.
	activePool := defaultPool
	if zone.TrapProbabilities != nil && len(zone.TrapProbabilities.TrapPool) > 0 {
		activePool = zone.TrapProbabilities.TrapPool
	}

	for _, room := range zone.Rooms {
		dangerLevel := room.DangerLevel
		if dangerLevel == "" {
			dangerLevel = zone.DangerLevel
		}

		probs := DefaultProbabilities[dangerLevel]
		roomChance := probs[0]
		coverChance := probs[1]

		// Override with zone-level probabilities if set.
		if zone.TrapProbabilities != nil {
			if zone.TrapProbabilities.RoomTrapChance != nil {
				roomChance = *zone.TrapProbabilities.RoomTrapChance
			}
			if zone.TrapProbabilities.CoverTrapChance != nil {
				coverChance = *zone.TrapProbabilities.CoverTrapChance
			}
		}
		// Override with room-level integer percentages if set (legacy fields).
		if room.RoomTrapChance != nil {
			roomChance = float64(*room.RoomTrapChance) / 100.0
		}
		if room.CoverTrapChance != nil {
			coverChance = float64(*room.CoverTrapChance) / 100.0
		}

		// Register static room-level traps (REQ-TR-9: never overwrite).
		staticRoomTrapPlaced := false
		for _, tc := range room.Traps {
			if tc.Position == "room" {
				instanceID := trapInstanceID(zone.ID, room.ID, "room", uuid.NewString())
				mgr.AddTrap(instanceID, tc.TemplateID, true)
				staticRoomTrapPlaced = true
			}
		}

		// Procedural room-level trap (only if no static room trap exists — REQ-TR-9).
		if !staticRoomTrapPlaced && dangerLevel != "safe" && roomChance > 0 {
			if rng.Float64() < roomChance {
				templateID := selectFromPool(activePool, rng)
				if templateID != "" {
					tmpl, ok := templates[templateID]
					if !ok || tmpl.ResetMode == ResetManual {
						// REQ-TR-10: procedural traps MUST use one_shot or auto only.
						// Skip any manual-reset template that leaked into the pool.
					} else {
						instanceID := trapInstanceID(zone.ID, room.ID, "room", uuid.NewString())
						mgr.AddTrap(instanceID, templateID, true)
					}
				}
			}
		}

		// Register static equipment-level traps; then roll for procedural cover traps.
		for i := range room.Equipment {
			eq := &room.Equipment[i]
			if eq.TrapTemplate != "" {
				// Static equipment trap (REQ-TR-9: already defined in YAML).
				instanceID := trapInstanceID(zone.ID, room.ID, "equip", eq.Description)
				mgr.AddTrap(instanceID, eq.TrapTemplate, true)
				continue
			}
			// Roll for procedural cover trap (only for cover items).
			if eq.CoverTier == "" {
				continue
			}
			if coverChance > 0 && rng.Float64() < coverChance {
				templateID := selectFromPool(activePool, rng)
				if templateID != "" {
					tmpl, ok := templates[templateID]
					if ok && tmpl.ResetMode != ResetManual { // REQ-TR-10: no manual reset
						eq.TrapTemplate = templateID
						instanceID := trapInstanceID(zone.ID, room.ID, "equip", eq.Description)
						mgr.AddTrap(instanceID, templateID, true)
					}
				}
			}
		}
	}
}
