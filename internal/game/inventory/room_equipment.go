package inventory

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/cory-johannsen/mud/internal/game/world"
)

// EquipmentInstance is a live instance of room equipment.
type EquipmentInstance struct {
	InstanceID string
	ItemDefID  string
	RoomID     string
	Immovable  bool
	Script     string
	configIdx  int
}

// respawnEntry tracks a pending respawn.
type respawnEntry struct {
	roomID    string
	configIdx int
	at        time.Time
	itemDefID string
	immovable bool
	script    string
}

// RoomEquipmentManager manages live equipment instances in rooms.
//
// Precondition: NewRoomEquipmentManager must be called before use.
// Postcondition: All methods are safe for concurrent use.
type RoomEquipmentManager struct {
	mu       sync.RWMutex
	rooms    map[string][]*EquipmentInstance
	configs  map[string][]world.RoomEquipmentConfig
	respawns []respawnEntry
}

// NewRoomEquipmentManager creates an empty manager.
func NewRoomEquipmentManager() *RoomEquipmentManager {
	return &RoomEquipmentManager{
		rooms:   make(map[string][]*EquipmentInstance),
		configs: make(map[string][]world.RoomEquipmentConfig),
	}
}

// InitRoom spawns initial instances for a room based on its equipment configs.
//
// Precondition: roomID must be non-empty; configs may be nil.
// Postcondition: Room is seeded with MaxCount instances per config.
func (m *RoomEquipmentManager) InitRoom(roomID string, configs []world.RoomEquipmentConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.configs[roomID] = configs
	m.rooms[roomID] = nil

	for idx, cfg := range configs {
		for i := 0; i < cfg.MaxCount; i++ {
			m.rooms[roomID] = append(m.rooms[roomID], &EquipmentInstance{
				InstanceID: uuid.New().String(),
				ItemDefID:  cfg.ItemID,
				RoomID:     roomID,
				Immovable:  cfg.Immovable,
				Script:     cfg.Script,
				configIdx:  idx,
			})
		}
	}
}

// EquipmentInRoom returns a snapshot of live equipment instances in a room.
//
// Precondition: roomID may be any string.
// Postcondition: Returns a non-nil slice (may be empty).
func (m *RoomEquipmentManager) EquipmentInRoom(roomID string) []*EquipmentInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := m.rooms[roomID]
	if len(items) == 0 {
		return []*EquipmentInstance{}
	}
	cp := make([]*EquipmentInstance, len(items))
	copy(cp, items)
	return cp
}

// GetInstance returns the instance with the given ID in the given room, or nil.
// The query is matched against InstanceID (UUID) first, then ItemDefID (item name),
// so players can type either the UUID or the human-readable item_id shown in the room.
//
// Precondition: roomID and instanceID may be any string.
// Postcondition: Returns nil if not found.
func (m *RoomEquipmentManager) GetInstance(roomID, instanceID string) *EquipmentInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var byItemDef *EquipmentInstance
	for _, it := range m.rooms[roomID] {
		if it.InstanceID == instanceID {
			cp := *it
			return &cp
		}
		if byItemDef == nil && it.ItemDefID == instanceID {
			byItemDef = it
		}
	}
	if byItemDef != nil {
		cp := *byItemDef
		return &cp
	}
	return nil
}

// Pickup removes a movable instance from the room and schedules respawn if configured.
// Returns false if instanceID is not found or item is immovable.
//
// Precondition: roomID and instanceID must be non-empty.
// Postcondition: If returned true, instance is removed and respawn scheduled when RespawnAfter > 0.
func (m *RoomEquipmentManager) Pickup(roomID, instanceID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := m.rooms[roomID]
	for i, it := range items {
		if it.InstanceID != instanceID {
			continue
		}
		if it.Immovable {
			return false
		}
		m.rooms[roomID] = append(items[:i], items[i+1:]...)
		cfg := m.configs[roomID]
		if it.configIdx < len(cfg) && cfg[it.configIdx].RespawnAfter > 0 {
			m.respawns = append(m.respawns, respawnEntry{
				roomID:    roomID,
				configIdx: it.configIdx,
				at:        time.Now().Add(cfg[it.configIdx].RespawnAfter),
				itemDefID: it.ItemDefID,
				immovable: it.Immovable,
				script:    it.Script,
			})
		}
		return true
	}
	return false
}

// ProcessRespawns spawns new instances for any pending respawn entries whose time has come.
//
// Precondition: none.
// Postcondition: Expired respawn entries are removed; new instances are added up to MaxCount.
func (m *RoomEquipmentManager) ProcessRespawns() {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	remaining := m.respawns[:0]
	for _, r := range m.respawns {
		if r.at.After(now) {
			remaining = append(remaining, r)
			continue
		}
		live := 0
		for _, it := range m.rooms[r.roomID] {
			if it.configIdx == r.configIdx {
				live++
			}
		}
		cfg := m.configs[r.roomID]
		if r.configIdx < len(cfg) && live < cfg[r.configIdx].MaxCount {
			m.rooms[r.roomID] = append(m.rooms[r.roomID], &EquipmentInstance{
				InstanceID: uuid.New().String(),
				ItemDefID:  r.itemDefID,
				RoomID:     r.roomID,
				Immovable:  r.immovable,
				Script:     r.script,
				configIdx:  r.configIdx,
			})
		}
	}
	m.respawns = remaining
}

// AddConfig adds a new equipment config to a room at runtime.
//
// Precondition: roomID must be non-empty; cfg.ItemID must be non-empty.
// Postcondition: Config is added; initial instances are spawned up to MaxCount.
func (m *RoomEquipmentManager) AddConfig(roomID string, cfg world.RoomEquipmentConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := len(m.configs[roomID])
	m.configs[roomID] = append(m.configs[roomID], cfg)
	for i := 0; i < cfg.MaxCount; i++ {
		m.rooms[roomID] = append(m.rooms[roomID], &EquipmentInstance{
			InstanceID: uuid.New().String(),
			ItemDefID:  cfg.ItemID,
			RoomID:     roomID,
			Immovable:  cfg.Immovable,
			Script:     cfg.Script,
			configIdx:  idx,
		})
	}
}

// RemoveConfig removes all instances of a config by item_id and removes the config.
//
// Precondition: roomID and itemID must be non-empty.
// Postcondition: All instances with matching ItemDefID are removed; returns false if not found.
func (m *RoomEquipmentManager) RemoveConfig(roomID, itemID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	var newCfgs []world.RoomEquipmentConfig
	found := false
	for _, c := range m.configs[roomID] {
		if c.ItemID == itemID {
			found = true
		} else {
			newCfgs = append(newCfgs, c)
		}
	}
	if !found {
		return false
	}
	m.configs[roomID] = newCfgs

	var newItems []*EquipmentInstance
	for _, it := range m.rooms[roomID] {
		if it.ItemDefID != itemID {
			newItems = append(newItems, it)
		}
	}
	m.rooms[roomID] = newItems

	// Cancel any pending respawns for this item to prevent stale configIdx hazard.
	var newRespawns []respawnEntry
	for _, r := range m.respawns {
		if r.roomID != roomID || r.itemDefID != itemID {
			newRespawns = append(newRespawns, r)
		}
	}
	m.respawns = newRespawns

	return true
}

// ListConfigs returns the equipment configs for a room.
//
// Precondition: roomID may be any string.
// Postcondition: Returns a non-nil slice (may be empty).
func (m *RoomEquipmentManager) ListConfigs(roomID string) []world.RoomEquipmentConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfgs := m.configs[roomID]
	cp := make([]world.RoomEquipmentConfig, len(cfgs))
	copy(cp, cfgs)
	return cp
}
