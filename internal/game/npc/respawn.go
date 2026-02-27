package npc

import (
	"sync"
	"time"
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
type RespawnManager struct {
	mu        sync.Mutex
	spawns    map[string][]RoomSpawn // roomID → configs
	templates map[string]*Template   // templateID → Template
	pending   []respawnEntry
}

// NewRespawnManager creates a RespawnManager from room spawn configs and a template map.
//
// Precondition: spawns and templates may be nil (manager becomes a no-op).
// Postcondition: Returns a non-nil RespawnManager.
func NewRespawnManager(spawns map[string][]RoomSpawn, templates map[string]*Template) *RespawnManager {
	if spawns == nil {
		spawns = make(map[string][]RoomSpawn)
	}
	if templates == nil {
		templates = make(map[string]*Template)
	}
	return &RespawnManager{
		spawns:    spawns,
		templates: templates,
	}
}

// PopulateRoom enforces the population cap for each RoomSpawn config in roomID.
// It first removes excess instances when the live count exceeds Max, then spawns
// new instances to fill the room up to exactly Max.
//
// Precondition: roomID must be non-empty; mgr must not be nil.
// Postcondition: for each template config in roomID, excess instances (count > Max)
// are removed so that count == Max, and when count < Max new instances are spawned
// until count == Max (subject to Spawn succeeding); the final live count equals
// exactly Max when all spawns succeed.
func (r *RespawnManager) PopulateRoom(roomID string, mgr *Manager) {
	r.mu.Lock()
	configs := append([]RoomSpawn(nil), r.spawns[roomID]...)
	r.mu.Unlock()

	for _, cfg := range configs {
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

// Schedule enqueues a future respawn for templateID in roomID after delay.
// No-op when delay == 0 (template does not respawn).
//
// Precondition: templateID and roomID must be non-empty.
// Postcondition: entry is added to pending iff delay > 0.
func (r *RespawnManager) Schedule(templateID, roomID string, delay time.Duration) {
	if delay <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// Subtract a small scheduling margin so that Tick called at (capturedNow + delay)
	// reliably fires entries scheduled at approximately capturedNow. The margin
	// (1ms) is far smaller than any meaningful delay but large enough to absorb
	// the nanosecond-scale gap between a caller capturing "now" and Schedule
	// recording its own time.Now().
	const schedulingMargin = time.Millisecond
	r.pending = append(r.pending, respawnEntry{
		templateID: templateID,
		roomID:     roomID,
		readyAt:    time.Now().Add(delay - schedulingMargin),
	})
}

// Tick drains all entries whose readyAt <= now, checks the population cap for
// each, and spawns up to the remaining capacity.
//
// Precondition: mgr must not be nil.
// Postcondition: pending entries with readyAt <= now are consumed.
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
		tmpl, ok := r.templates[e.templateID]
		if !ok {
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
		_, _ = mgr.Spawn(tmpl, e.roomID)
	}
}

// ResolvedDelay returns the effective respawn delay for templateID in roomID:
// the room's RespawnDelay if non-zero, otherwise the template's parsed RespawnDelay.
// Returns 0 when neither is set or the template is unknown.
//
// Postcondition: Returns >= 0.
func (r *RespawnManager) ResolvedDelay(templateID, roomID string) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
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
	r.mu.Lock()
	defer r.mu.Unlock()
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
