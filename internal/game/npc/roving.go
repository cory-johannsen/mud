package npc

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/cory-johannsen/mud/internal/game/world"
)

// WorldRoomReader is the subset of world.Manager needed by RovingManager.
//
// Precondition: Implementations must be safe for concurrent use.
type WorldRoomReader interface {
	GetRoom(id string) (*world.Room, bool)
}

// RovingMoveFunc is called by RovingManager after moving an NPC.
// The caller is responsible for broadcasting notifications.
// fromRoomID and toRoomID are the room IDs before and after the move.
//
// Precondition: instID, fromRoomID, toRoomID are all non-empty.
type RovingMoveFunc func(instID, fromRoomID, toRoomID string)

// rovingEntry holds per-NPC tracking state for the RovingManager.
// All mutable timing fields are owned by rovingEntry and protected by rm.mu.
type rovingEntry struct {
	instID      string
	tmpl        *Template // non-nil; Roving field is guaranteed non-nil
	route       []string  // copy of RovingConfig.Route
	routeIdx    int       // current index; replaces inst.RovingRouteIndex
	routeDir    int       // +1 or -1; replaces inst.RovingRouteDir
	nextMoveAt  time.Time // replaces inst.RovingNextMoveAt
	pausedUntil time.Time // replaces inst.RovingPausedUntil
}

// RovingManager drives autonomous multi-zone NPC movement.
// It maintains a registry of roving NPC instance IDs and advances their
// positions on each internal tick.
//
// Invariant: All methods are safe for concurrent use.
type RovingManager struct {
	mu       sync.Mutex
	entries  map[string]*rovingEntry // instID → entry
	npcMgr   *Manager
	world    WorldRoomReader
	onMove   RovingMoveFunc
	tickRate time.Duration
}

// NewRovingManager constructs a RovingManager.
//
// Precondition: npcMgr must not be nil; world must not be nil; onMove must not be nil.
// Postcondition: Returns a non-nil *RovingManager ready for use; tickRate is 15 seconds.
func NewRovingManager(npcMgr *Manager, world WorldRoomReader, onMove RovingMoveFunc) *RovingManager {
	return &RovingManager{
		entries:  make(map[string]*rovingEntry),
		npcMgr:   npcMgr,
		world:    world,
		onMove:   onMove,
		tickRate: 15 * time.Second,
	}
}

// Register adds a roving NPC to the manager's tracking set.
// No-op if inst is nil, tmpl is nil, or tmpl.Roving is nil.
//
// Precondition: inst.ID must be non-empty if inst is non-nil.
// Postcondition: The NPC will be ticked on subsequent calls to Tick().
func (rm *RovingManager) Register(inst *Instance, tmpl *Template) {
	if inst == nil || tmpl == nil || tmpl.Roving == nil {
		return
	}
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.entries[inst.ID] = &rovingEntry{
		instID:     inst.ID,
		tmpl:       tmpl,
		route:      append([]string(nil), tmpl.Roving.Route...),
		routeIdx:   inst.RovingRouteIndex,
		routeDir:   inst.RovingRouteDir,
		nextMoveAt: inst.RovingNextMoveAt,
		// pausedUntil: zero (no initial pause)
	}
}

// Unregister removes an NPC from tracking (e.g. on death).
// No-op if the instID is not tracked.
//
// Precondition: instID may be any string.
// Postcondition: The NPC will no longer be ticked.
func (rm *RovingManager) Unregister(instID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.entries, instID)
}

// PauseFor sets the NPC's pausedUntil to now+d, blocking movement.
// Called on combat entry. No-op if the NPC is not tracked.
// A negative duration effectively clears the pause (sets pausedUntil to a past time).
//
// Precondition: d must be positive for a meaningful pause.
// Postcondition: The NPC will not move until time.Now() >= pausedUntil.
func (rm *RovingManager) PauseFor(instID string, d time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	entry, ok := rm.entries[instID]
	if !ok {
		return
	}
	entry.pausedUntil = time.Now().Add(d)
}

// Start begins the background tick goroutine. Blocks until ctx is cancelled.
//
// Precondition: ctx must be non-nil.
// Postcondition: Returns when ctx.Done() is closed.
func (rm *RovingManager) Start(ctx context.Context) {
	ticker := time.NewTicker(rm.tickRate)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			rm.Tick(now)
		}
	}
}

// Tick advances all tracked NPCs whose move timers have elapsed.
//
// Precondition: now must represent the current tick time.
func (rm *RovingManager) Tick(now time.Time) {
	rm.mu.Lock()
	// Snapshot entry IDs to avoid holding mu during potentially slow npcMgr calls.
	ids := make([]string, 0, len(rm.entries))
	for id := range rm.entries {
		ids = append(ids, id)
	}
	rm.mu.Unlock()

	for _, instID := range ids {
		rm.mu.Lock()
		entry, ok := rm.entries[instID]
		if !ok {
			rm.mu.Unlock()
			continue
		}
		// Check timing under lock (no data race).
		if now.Before(entry.nextMoveAt) || now.Before(entry.pausedUntil) {
			rm.mu.Unlock()
			continue
		}
		rm.mu.Unlock()

		inst, alive := rm.npcMgr.Get(instID)
		if !alive || inst.IsDead() {
			rm.Unregister(instID)
			continue
		}

		currentRoom, ok := rm.world.GetRoom(inst.RoomID)
		if !ok {
			continue
		}

		var nextRoomID string
		rm.mu.Lock()
		// Re-fetch entry after lock (may have changed).
		entry, ok = rm.entries[instID]
		if !ok {
			rm.mu.Unlock()
			continue
		}
		if len(currentRoom.Exits) > 0 && rand.Float64() < entry.tmpl.Roving.ExploreProbability {
			exit := currentRoom.Exits[rand.Intn(len(currentRoom.Exits))]
			nextRoomID = exit.TargetRoom
		} else {
			nextRoomID, entry.routeIdx, entry.routeDir = nextRouteRoom(entry.route, entry.routeIdx, entry.routeDir)
		}
		// Advance the move timer before calling Move so that a failed move still
		// consumes the travel interval; the NPC will retry on the next tick.
		entry.nextMoveAt = now.Add(parseTravelInterval(entry.tmpl.Roving.TravelInterval))
		rm.mu.Unlock()

		if nextRoomID == "" {
			continue
		}

		fromRoom := inst.RoomID
		if err := rm.npcMgr.Move(instID, nextRoomID); err != nil {
			continue
		}
		rm.onMove(instID, fromRoom, nextRoomID)
	}
}

// nextRouteRoom computes the next room ID for route-following traversal.
// It implements bounce (ping-pong) traversal: forward to end, then backward.
// Returns the next room ID, updated index, and updated direction.
//
// Precondition: route must be non-empty; dir must be +1 or -1; idx must be in [0, len(route)-1].
// Postcondition: Returns a non-empty room ID and valid (idx, dir) pair.
func nextRouteRoom(route []string, idx, dir int) (roomID string, newIdx, newDir int) {
	if len(route) == 0 {
		return "", idx, dir
	}
	if len(route) == 1 {
		return route[0], 0, 1
	}

	next := idx + dir
	if next >= len(route) {
		dir = -1
		next = len(route) - 2
		if next < 0 {
			next = 0
		}
	} else if next < 0 {
		dir = 1
		next = 1
		if next >= len(route) {
			next = 0
		}
	}
	return route[next], next, dir
}
