// Package world: editor.go provides atomic YAML write and hot-reload for world editing.
package world

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/danger"
)

// WorldEditor encapsulates atomic YAML write and hot-reload operations for world editing.
//
// Invariant: contentDir is writable at construction time.
// Concurrency: All methods are safe for concurrent use via the Manager's internal locking.
type WorldEditor struct {
	contentDir string
	manager    *Manager
}

// NewWorldEditor creates a WorldEditor that writes zone files to contentDir and reloads via manager.
//
// Precondition: contentDir must be a writable directory path; manager must be non-nil.
// Postcondition: Returns (nil, error) if contentDir is not writable. Returns non-nil *WorldEditor on success.
func NewWorldEditor(contentDir string, manager *Manager) (*WorldEditor, error) {
	probe := filepath.Join(contentDir, ".write_probe")
	if err := os.WriteFile(probe, []byte("probe"), 0600); err != nil {
		return nil, fmt.Errorf("content dir %q is not writable: %w", contentDir, err)
	}
	_ = os.Remove(probe)
	return &WorldEditor{contentDir: contentDir, manager: manager}, nil
}

// AddRoom appends a new room to an existing zone, writes atomically, and hot-reloads.
//
// Precondition: zoneID must match a loaded zone; roomID must not already exist in the zone.
// Postcondition: On success, the zone is persisted and the manager reflects the new room.
func (e *WorldEditor) AddRoom(zoneID, roomID, title string) error {
	zone, ok := e.manager.GetZone(zoneID)
	if !ok {
		return fmt.Errorf("unknown zone: %s", zoneID)
	}
	if _, exists := zone.Rooms[roomID]; exists {
		return fmt.Errorf("room ID %s already exists in zone %s", roomID, zoneID)
	}

	// Compute map position: max(MapX)+1 for x; MapY of the room with max MapX.
	maxX := -1
	mapY := 0
	for _, r := range zone.Rooms {
		if r.MapX > maxX {
			maxX = r.MapX
			mapY = r.MapY
		}
	}
	newX := maxX + 1
	if maxX < 0 {
		newX = 0
		mapY = 0
	}

	newRoom := &Room{
		ID:          roomID,
		ZoneID:      zoneID,
		Title:       title,
		Description: "",
		MapX:        newX,
		MapY:        mapY,
		Properties:  make(map[string]string),
	}
	zone.Rooms[roomID] = newRoom

	if err := e.writeZoneAtomic(zone); err != nil {
		delete(zone.Rooms, roomID)
		return err
	}
	return e.manager.ReloadZone(zone)
}

// AddLink adds a bidirectional exit between two rooms, writes affected zone(s) atomically, and hot-reloads each.
//
// Precondition: Both rooms must exist; direction must be a standard direction; neither direction slot may be occupied.
// Postcondition: On success, both rooms persist the new exits and the manager is updated.
func (e *WorldEditor) AddLink(fromRoomID, directionStr, toRoomID string) error {
	dir := Direction(directionStr)
	if !dir.IsStandard() {
		return fmt.Errorf("invalid direction: %s. Valid: north, south, east, west, northeast, northwest, southeast, southwest, up, down", directionStr)
	}
	fromRoom, ok := e.manager.GetRoom(fromRoomID)
	if !ok {
		return fmt.Errorf("unknown room: %s", fromRoomID)
	}
	toRoom, ok := e.manager.GetRoom(toRoomID)
	if !ok {
		return fmt.Errorf("unknown room: %s", toRoomID)
	}
	for _, exit := range fromRoom.Exits {
		if exit.Direction == dir {
			return fmt.Errorf("direction %s is already occupied in %s", directionStr, fromRoomID)
		}
	}
	rev := dir.Opposite()
	for _, exit := range toRoom.Exits {
		if exit.Direction == rev {
			return fmt.Errorf("reverse direction %s is already occupied in %s", string(rev), toRoomID)
		}
	}

	fromRoom.Exits = append(fromRoom.Exits, Exit{Direction: dir, TargetRoom: toRoomID})
	toRoom.Exits = append(toRoom.Exits, Exit{Direction: rev, TargetRoom: fromRoomID})

	fromZone, _ := e.manager.GetZone(fromRoom.ZoneID)
	toZone, _ := e.manager.GetZone(toRoom.ZoneID)

	if fromZone.ID == toZone.ID {
		// REQ-EC-20: single write and reload for same-zone link.
		if err := e.writeZoneAtomic(fromZone); err != nil {
			fromRoom.Exits = fromRoom.Exits[:len(fromRoom.Exits)-1]
			toRoom.Exits = toRoom.Exits[:len(toRoom.Exits)-1]
			return err
		}
		return e.manager.ReloadZone(fromZone)
	}

	if err := e.writeZoneAtomic(fromZone); err != nil {
		fromRoom.Exits = fromRoom.Exits[:len(fromRoom.Exits)-1]
		toRoom.Exits = toRoom.Exits[:len(toRoom.Exits)-1]
		return err
	}
	if err := e.manager.ReloadZone(fromZone); err != nil {
		return err
	}
	if err := e.writeZoneAtomic(toZone); err != nil {
		toRoom.Exits = toRoom.Exits[:len(toRoom.Exits)-1]
		return err
	}
	return e.manager.ReloadZone(toZone)
}

// RemoveLink removes a single directional exit from a room, writes atomically, and hot-reloads.
//
// Precondition: roomID must exist; direction must be present in the room's exits.
// Postcondition: On success, the exit is removed, persisted, and manager is updated.
func (e *WorldEditor) RemoveLink(roomID, directionStr string) error {
	room, ok := e.manager.GetRoom(roomID)
	if !ok {
		return fmt.Errorf("unknown room: %s", roomID)
	}
	dir := Direction(directionStr)
	found := -1
	for i, exit := range room.Exits {
		if exit.Direction == dir {
			found = i
			break
		}
	}
	if found < 0 {
		return fmt.Errorf("no %s exit exists in %s", directionStr, roomID)
	}

	removed := room.Exits[found]
	room.Exits = append(room.Exits[:found], room.Exits[found+1:]...)

	zone, _ := e.manager.GetZone(room.ZoneID)
	if err := e.writeZoneAtomic(zone); err != nil {
		// Undo.
		newExits := make([]Exit, len(room.Exits)+1)
		copy(newExits, room.Exits[:found])
		newExits[found] = removed
		copy(newExits[found+1:], room.Exits[found:])
		room.Exits = newExits
		return err
	}
	return e.manager.ReloadZone(zone)
}

// SetRoomField sets a supported field on the specified room, writes atomically, and hot-reloads.
//
// Supported fields: title, description, danger_level.
// Precondition: roomID must exist; field and value must be valid per field constraints.
// Postcondition: On success, the room field is updated, persisted, and manager is updated.
func (e *WorldEditor) SetRoomField(roomID, field, value string) error {
	room, ok := e.manager.GetRoom(roomID)
	if !ok {
		return fmt.Errorf("unknown room: %s", roomID)
	}

	switch field {
	case "title":
		if value == "" {
			return fmt.Errorf("title cannot be empty")
		}
		room.Title = value
	case "description":
		if value == "" {
			return fmt.Errorf("description cannot be empty")
		}
		room.Description = value
	case "danger_level":
		switch danger.DangerLevel(value) {
		case danger.Safe, danger.Sketchy, danger.Dangerous, danger.AllOutWar:
			// valid
		default:
			return fmt.Errorf("invalid danger level: %s. Valid: safe, sketchy, dangerous, all_out_war", value)
		}
		room.DangerLevel = value
	default:
		return fmt.Errorf("unknown field: %s. Valid fields: title, description, danger_level", field)
	}

	zone, _ := e.manager.GetZone(room.ZoneID)
	if err := e.writeZoneAtomic(zone); err != nil {
		return err
	}
	return e.manager.ReloadZone(zone)
}

// writeZoneAtomic serializes zone to its YAML file atomically.
//
// Precondition: zone must be non-nil; zone.ID must correspond to a file in e.contentDir.
// Postcondition: The target YAML file is replaced atomically. Temp file is removed on any error.
func (e *WorldEditor) writeZoneAtomic(zone *Zone) error {
	targetPath := filepath.Join(e.contentDir, zone.ID+".yaml")

	yf := zoneToYAML(zone)
	data, err := yaml.Marshal(yf)
	if err != nil {
		return fmt.Errorf("marshaling zone %s: %w", zone.ID, err)
	}

	tmp, err := os.CreateTemp(e.contentDir, ".zone_write_*")
	if err != nil {
		return fmt.Errorf("creating temp file for zone %s: %w", zone.ID, err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing zone %s: %w", zone.ID, err)
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("syncing zone %s: %w", zone.ID, err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("closing temp for zone %s: %w", zone.ID, err)
	}
	if err = os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("renaming zone %s: %w", zone.ID, err)
	}
	return nil
}
