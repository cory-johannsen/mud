package npc

import (
	"sync"
	"time"

	"github.com/cory-johannsen/mud/internal/game/npc/behavior"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// RoomSpawn holds the resolved spawn configuration for one NPC template in one room.
//
// Invariant: Max >= 1; RespawnDelay == 0 means this template does not respawn.
type RoomSpawn struct {
	// TemplateID is the NPC template to spawn.
	TemplateID string
	// Max is the population cap: respawn is suppressed when live count >= Max.
	Max int
	// RespawnDelay is the duration to wait before attempting a respawn.
	// Zero means the template does not respawn.
	RespawnDelay time.Duration
}

// respawnEntry represents a single pending respawn.
type respawnEntry struct {
	templateID string
	roomID     string
	readyAt    time.Time
}

// RespawnManager schedules and executes NPC respawns.
// It is safe for concurrent use.
//
// Invariant: entries with zero delay are never queued.
//
// Concurrency: Tick and PopulateRoom must not be called concurrently with each
// other or with themselves. Schedule may be called concurrently from any goroutine.
// In practice, PopulateRoom is called only during single-threaded startup and
// Tick is driven by a single ZoneTickManager goroutine per zone.
type RespawnManager struct {
	mu        sync.RWMutex
	spawns    map[string][]RoomSpawn    // roomID → configs
	templates map[string]*Template      // templateID → Template
	pending   []respawnEntry
	bossRooms map[string]bool // set of roomIDs marked as boss_room
	// zoneRooms maps zoneID → ordered room slice for BFS computation. REQ-NB-38.
	zoneRooms map[string][]*world.Room
	// roomToZone maps roomID → zoneID. REQ-NB-38.
	roomToZone map[string]string
}

// NewRespawnManager creates a RespawnManager from room spawn configs and a template map.
//
// Precondition: spawns and templates may be nil (manager becomes a no-op).
// zoneRooms and roomToZone may be nil; when nil, HomeRoomBFS is not populated on respawn.
// Postcondition: Returns a non-nil RespawnManager.
func NewRespawnManager(spawns map[string][]RoomSpawn, templates map[string]*Template, zoneRooms map[string][]*world.Room, roomToZone map[string]string) *RespawnManager {
	return NewRespawnManagerWithBossRooms(spawns, templates, nil, zoneRooms, roomToZone)
}

// NewRespawnManagerWithBossRooms creates a RespawnManager that knows which rooms are boss rooms.
// When a boss-tier NPC respawns in a boss room, CoordinatedBossRespawn is triggered.
//
// Precondition: spawns, templates, bossRooms, zoneRooms, and roomToZone may be nil (treated as empty maps).
// Postcondition: Returns a non-nil RespawnManager.
func NewRespawnManagerWithBossRooms(spawns map[string][]RoomSpawn, templates map[string]*Template, bossRooms map[string]bool, zoneRooms map[string][]*world.Room, roomToZone map[string]string) *RespawnManager {
	if spawns == nil {
		spawns = make(map[string][]RoomSpawn)
	}
	if templates == nil {
		templates = make(map[string]*Template)
	}
	if bossRooms == nil {
		bossRooms = make(map[string]bool)
	}
	if zoneRooms == nil {
		zoneRooms = make(map[string][]*world.Room)
	}
	if roomToZone == nil {
		roomToZone = make(map[string]string)
	}
	return &RespawnManager{
		spawns:     spawns,
		templates:  templates,
		bossRooms:  bossRooms,
		zoneRooms:  zoneRooms,
		roomToZone: roomToZone,
	}
}

// PendingCount returns the number of pending respawn entries for the given roomID.
//
// Precondition: roomID must be non-empty for a meaningful result.
// Postcondition: Returns the count of pending entries whose roomID matches.
func (r *RespawnManager) PendingCount(roomID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, e := range r.pending {
		if e.roomID == roomID {
			count++
		}
	}
	return count
}

// CoordinatedBossRespawn cancels all pending individual respawn timers for roomID
// and immediately triggers PopulateRoom for all spawn configs in that room.
// Called when a boss-tier NPC respawns in a boss_room.
//
// Precondition: roomID must be non-empty; mgr must not be nil.
// Postcondition: All pending timers for roomID are removed; all spawns repopulated.
func (r *RespawnManager) CoordinatedBossRespawn(roomID string, mgr *Manager) {
	r.mu.Lock()
	filtered := r.pending[:0]
	for _, e := range r.pending {
		if e.roomID != roomID {
			filtered = append(filtered, e)
		}
	}
	r.pending = filtered
	r.mu.Unlock()

	r.PopulateRoom(roomID, mgr)
}

// GetTemplate returns the NPC template with the given id, or (nil, false) if not found.
//
// Precondition: id must be non-empty for a meaningful result.
// Postcondition: Returned *Template is non-nil when ok is true.
func (r *RespawnManager) GetTemplate(id string) (*Template, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.templates[id]
	return t, ok
}

// PopulateRoom enforces the population cap for each RoomSpawn config in roomID.
// It first removes excess instances when the live count exceeds Max, then spawns
// new instances to fill the room up to exactly Max.
//
// Precondition: roomID must be non-empty; mgr must not be nil.
// Postcondition: for each template config in roomID, instances beyond Max are removed
// and new instances are spawned until count == Max (subject to Spawn succeeding).
// This method must not be called concurrently with Tick or other PopulateRoom calls.
func (r *RespawnManager) PopulateRoom(roomID string, mgr *Manager) {
	r.mu.Lock()
	configs := append([]RoomSpawn(nil), r.spawns[roomID]...)
	r.mu.Unlock()

	for _, cfg := range configs {
		// r.templates is read-only after construction; no lock required.
		tmpl, ok := r.templates[cfg.TemplateID]
		if !ok {
			continue
		}

		// Remove excess instances when count exceeds cap.
		instances := mgr.InstancesInRoom(roomID)
		var matching []*Instance
		for _, inst := range instances {
			if inst.TemplateID == cfg.TemplateID {
				matching = append(matching, inst)
			}
		}
		for len(matching) > cfg.Max {
			last := matching[len(matching)-1]
			matching = matching[:len(matching)-1]
			_ = mgr.Remove(last.ID)
		}

		// Spawn to fill up to cap.
		current := len(matching)
		for i := current; i < cfg.Max; i++ {
			if _, err := mgr.Spawn(tmpl, roomID); err != nil {
				// Spawn failure is non-fatal; the next PopulateRoom call will retry.
				continue
			}
		}
	}
}

// Schedule enqueues a future respawn for templateID in roomID to fire at now+delay.
// No-op when delay == 0 (template does not respawn).
//
// Precondition: templateID and roomID must be non-empty; now must be a valid time.
// Postcondition: entry is added to pending with readyAt = now+delay iff delay > 0.
func (r *RespawnManager) Schedule(templateID, roomID string, now time.Time, delay time.Duration) {
	if delay <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending = append(r.pending, respawnEntry{
		templateID: templateID,
		roomID:     roomID,
		readyAt:    now.Add(delay),
	})
}

// Tick drains all entries whose readyAt <= now, checks the population cap for
// each, and spawns up to the remaining capacity.
//
// Precondition: mgr must not be nil.
// Postcondition: pending entries with readyAt <= now are consumed.
// This method must not be called concurrently with other Tick or PopulateRoom calls.
func (r *RespawnManager) Tick(now time.Time, mgr *Manager) {
	r.mu.Lock()
	var ready, future []respawnEntry
	for _, e := range r.pending {
		if !e.readyAt.After(now) {
			ready = append(ready, e)
		} else {
			future = append(future, e)
		}
	}
	r.pending = future
	r.mu.Unlock()

	for _, e := range ready {
		// r.templates is read-only after construction; no lock required.
		tmpl, ok := r.templates[e.templateID]
		if !ok {
			continue
		}
		// If the respawning template is a boss tier and the room is a boss room,
		// use coordinated respawn instead of individual spawn.
		if tmpl.Tier == "boss" && r.bossRooms[e.roomID] {
			r.CoordinatedBossRespawn(e.roomID, mgr)
			continue
		}
		cfg, ok := r.configFor(e.roomID, e.templateID)
		if !ok {
			continue
		}
		current := r.countInRoom(e.roomID, e.templateID, mgr)
		if current >= cfg.Max {
			continue
		}
		inst, _ := mgr.Spawn(tmpl, e.roomID)
		if inst != nil && inst.HomeRoomID != "" {
			if zoneID, ok := r.roomToZone[inst.HomeRoomID]; ok {
				if rooms := r.zoneRooms[zoneID]; len(rooms) > 0 {
					if dm, err := behavior.BFSDistanceMap(rooms, inst.HomeRoomID); err == nil {
						inst.HomeRoomBFS = dm
					}
				}
			}
		}
	}
}

// ResolvedDelay returns the effective respawn delay for templateID in roomID:
// the room's RespawnDelay if non-zero, otherwise the template's parsed RespawnDelay.
// Returns 0 when neither is set or the template is unknown.
//
// Postcondition: Returns >= 0.
func (r *RespawnManager) ResolvedDelay(templateID, roomID string) time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, cfg := range r.spawns[roomID] {
		if cfg.TemplateID == templateID && cfg.RespawnDelay > 0 {
			return cfg.RespawnDelay
		}
	}
	tmpl, ok := r.templates[templateID]
	if !ok || tmpl.RespawnDelay == "" {
		return 0
	}
	d, err := time.ParseDuration(tmpl.RespawnDelay)
	if err != nil {
		return 0
	}
	return d
}

// configFor finds the RoomSpawn config for templateID in roomID.
// Caller must NOT hold r.mu.
func (r *RespawnManager) configFor(roomID, templateID string) (RoomSpawn, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, cfg := range r.spawns[roomID] {
		if cfg.TemplateID == templateID {
			return cfg, true
		}
	}
	return RoomSpawn{}, false
}

// countInRoom counts live instances of templateID in roomID.
func (r *RespawnManager) countInRoom(roomID, templateID string, mgr *Manager) int {
	instances := mgr.InstancesInRoom(roomID)
	count := 0
	for _, inst := range instances {
		if inst.TemplateID == templateID {
			count++
		}
	}
	return count
}
