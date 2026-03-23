package npc

import (
	"fmt"
	"time"

	"github.com/google/wire"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc/behavior"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// NPCsDir is the path to NPC template YAML files.
type NPCsDir string

// LoadTemplatesFromDir loads NPC templates from the given directory.
func LoadTemplatesFromDir(dir NPCsDir, logger *zap.Logger) ([]*Template, error) {
	templates, err := LoadTemplates(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading npc templates from %q: %w", dir, err)
	}
	logger.Info("loaded npc templates", zap.Int("count", len(templates)))
	return templates, nil
}

// NewWiredManager creates a Manager and wires the armor AC resolver from invRegistry.
func NewWiredManager(invRegistry *inventory.Registry) *Manager {
	mgr := NewManager()
	mgr.SetArmorACResolver(func(armorID string) int {
		if def, ok := invRegistry.Armor(armorID); ok {
			return def.ACBonus
		}
		return 0
	})
	return mgr
}

// NewPopulatedRespawnManager builds the respawn manager from zone spawn configs,
// populates initial NPC instances into npcMgr, and returns the manager.
func NewPopulatedRespawnManager(
	templates []*Template,
	worldMgr *world.Manager,
	npcMgr *Manager,
	logger *zap.Logger,
) (*RespawnManager, error) {
	templateByID := make(map[string]*Template, len(templates))
	for _, tmpl := range templates {
		templateByID[tmpl.ID] = tmpl
	}
	roomSpawns := make(map[string][]RoomSpawn)
	for _, zone := range worldMgr.AllZones() {
		for _, room := range zone.Rooms {
			for _, sc := range room.Spawns {
				tmpl, ok := templateByID[sc.Template]
				if !ok {
					return nil, fmt.Errorf("spawn in room %q references unknown npc template %q", room.ID, sc.Template)
				}
				var delay time.Duration
				if sc.RespawnAfter != "" {
					d, err := time.ParseDuration(sc.RespawnAfter)
					if err != nil {
						return nil, fmt.Errorf("invalid respawn_after %q in room %q: %w", sc.RespawnAfter, room.ID, err)
					}
					delay = d
				} else if tmpl.RespawnDelay != "" {
					d, err := time.ParseDuration(tmpl.RespawnDelay)
					if err != nil {
						return nil, fmt.Errorf("invalid respawn_delay %q on template %q: %w", tmpl.RespawnDelay, tmpl.ID, err)
					}
					delay = d
				}
				roomSpawns[room.ID] = append(roomSpawns[room.ID], RoomSpawn{
					TemplateID:   sc.Template,
					Max:          sc.Count,
					RespawnDelay: delay,
				})
			}
		}
	}
	// Build zone room slices for BFS computation. REQ-NB-38.
	zoneRooms := make(map[string][]*world.Room)
	for _, zone := range worldMgr.AllZones() {
		rooms := make([]*world.Room, 0, len(zone.Rooms))
		for _, r := range zone.Rooms {
			rooms = append(rooms, r)
		}
		zoneRooms[zone.ID] = rooms
	}
	roomToZone := make(map[string]string)
	for _, zone := range worldMgr.AllZones() {
		for _, r := range zone.Rooms {
			roomToZone[r.ID] = zone.ID
		}
	}

	respawnMgr := NewRespawnManager(roomSpawns, templateByID)
	for roomID := range roomSpawns {
		respawnMgr.PopulateRoom(roomID, npcMgr)
	}

	// Populate HomeRoomBFS for each spawned instance. REQ-NB-38.
	for _, inst := range npcMgr.AllInstances() {
		if inst.HomeRoomID == "" {
			continue
		}
		zoneID, ok := roomToZone[inst.HomeRoomID]
		if !ok {
			continue
		}
		rooms := zoneRooms[zoneID]
		dm, err := behavior.BFSDistanceMap(rooms, inst.HomeRoomID)
		if err != nil {
			logger.Warn("BFSDistanceMap failed for NPC home room",
				zap.String("npc_id", inst.ID),
				zap.String("home_room", inst.HomeRoomID),
				zap.Error(err))
			continue
		}
		inst.HomeRoomBFS = dm
	}

	logger.Info("initial NPC population complete", zap.Int("room_configs", len(roomSpawns)))
	return respawnMgr, nil
}

// Providers is the wire provider set for NPC dependencies.
var Providers = wire.NewSet(
	LoadTemplatesFromDir,
	NewWiredManager,
	NewPopulatedRespawnManager,
)
