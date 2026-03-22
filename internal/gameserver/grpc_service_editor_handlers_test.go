package gameserver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// evtText extracts the text content from either a MessageEvent or ErrorEvent in a ServerEvent.
func evtText(evt *gamev1.ServerEvent) string {
	if evt == nil {
		return ""
	}
	if msg := evt.GetMessage(); msg != nil {
		return msg.GetContent()
	}
	if errEvt := evt.GetError(); errEvt != nil {
		return errEvt.GetMessage()
	}
	return ""
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// setupEditorService creates a minimal GameServiceServer for editor handler tests,
// backed by a writable temp dir for WorldEditor.
func setupEditorService(t *testing.T) (*GameServiceServer, *world.Manager, *session.Manager, string) {
	t.Helper()
	dir := t.TempDir()
	zoneYAML := `zone:
  id: "test"
  name: "Test Zone"
  start_room: "r1"
  rooms:
    - id: "r1"
      title: "Room One"
      description: "A room."
      map_x: 0
      map_y: 0
      exits: []
    - id: "r2"
      title: "Room Two"
      description: "Another room."
      map_x: 1
      map_y: 0
      exits: []
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(zoneYAML), 0644))
	zones, err := world.LoadZonesFromDir(dir)
	require.NoError(t, err)
	worldMgr, err := world.NewManager(zones)
	require.NoError(t, err)

	sessMgr := session.NewManager()
	cmdRegistry := command.DefaultRegistry()
	npcMgr := npc.NewManager()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	svc := newTestGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)

	we, err := world.NewWorldEditor(dir, worldMgr)
	require.NoError(t, err)
	svc.worldEditor = we

	return svc, worldMgr, sessMgr, dir
}

// addEditorSession registers an editor-role session to sessMgr at a given room.
func addEditorSession(t *testing.T, sessMgr *session.Manager, uid, roomID string) {
	t.Helper()
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:     uid,
		Username: uid,
		CharName: uid,
		RoomID:  roomID,
		Role:    postgres.RoleEditor,
		CurrentHP: 10,
		MaxHP:     10,
	})
	require.NoError(t, err)
}

// addPlayerSession registers a player-role session.
func addPlayerSession(t *testing.T, sessMgr *session.Manager, uid, roomID string) {
	t.Helper()
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:     uid,
		Username: uid,
		CharName: uid,
		RoomID:  roomID,
		Role:    postgres.RolePlayer,
		CurrentHP: 10,
		MaxHP:     10,
	})
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// handleSpawnNPC tests
// ---------------------------------------------------------------------------

func TestHandleSpawnNPC_UnknownTemplate(t *testing.T) {
	svc, _, sessMgr, _ := setupEditorService(t)
	addEditorSession(t, sessMgr, "u1", "r1")

	svc.respawnMgr = npc.NewRespawnManager(nil, nil)
	evt, err := svc.handleSpawnNPC("u1", &gamev1.SpawnNPCRequest{TemplateId: "ghost", RoomId: "r1"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evtText(evt), "ghost")
}

func TestHandleSpawnNPC_UnknownRoom(t *testing.T) {
	svc, _, sessMgr, _ := setupEditorService(t)
	addEditorSession(t, sessMgr, "u1", "r1")

	tmpl := &npc.Template{ID: "guard", Name: "Guard", Level: 1}
	svc.respawnMgr = npc.NewRespawnManager(nil, map[string]*npc.Template{"guard": tmpl})
	evt, err := svc.handleSpawnNPC("u1", &gamev1.SpawnNPCRequest{TemplateId: "guard", RoomId: "no_such_room"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evtText(evt), "no_such_room")
}

func TestHandleSpawnNPC_DeniedPlayerRole(t *testing.T) {
	svc, _, sessMgr, _ := setupEditorService(t)
	addPlayerSession(t, sessMgr, "u1", "r1")

	tmpl := &npc.Template{ID: "guard", Name: "Guard", Level: 1}
	svc.respawnMgr = npc.NewRespawnManager(nil, map[string]*npc.Template{"guard": tmpl})
	evt, err := svc.handleSpawnNPC("u1", &gamev1.SpawnNPCRequest{TemplateId: "guard", RoomId: "r1"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evtText(evt), "permission denied")
}

// ---------------------------------------------------------------------------
// handleAddRoom tests
// ---------------------------------------------------------------------------

func TestHandleAddRoom_NoWorldEditor(t *testing.T) {
	svc, _, sessMgr, _ := setupEditorService(t)
	svc.worldEditor = nil
	addEditorSession(t, sessMgr, "u1", "r1")

	evt, err := svc.handleAddRoom("u1", &gamev1.AddRoomRequest{ZoneId: "test", RoomId: "r3", Title: "Room Three"})
	require.NoError(t, err)
	assert.Contains(t, evtText(evt), "not available")
}

func TestHandleAddRoom_UnknownZone(t *testing.T) {
	svc, _, sessMgr, _ := setupEditorService(t)
	addEditorSession(t, sessMgr, "u1", "r1")

	evt, err := svc.handleAddRoom("u1", &gamev1.AddRoomRequest{ZoneId: "nonexistent", RoomId: "r3", Title: "Room Three"})
	require.NoError(t, err)
	assert.Contains(t, evtText(evt), "unknown zone")
}

func TestHandleAddRoom_Success(t *testing.T) {
	svc, worldMgr, sessMgr, _ := setupEditorService(t)
	addEditorSession(t, sessMgr, "u1", "r1")

	evt, err := svc.handleAddRoom("u1", &gamev1.AddRoomRequest{ZoneId: "test", RoomId: "r3", Title: "Room Three"})
	require.NoError(t, err)
	assert.Contains(t, evtText(evt), "r3")

	_, ok := worldMgr.GetRoom("r3")
	assert.True(t, ok)
}

// ---------------------------------------------------------------------------
// handleAddLink tests
// ---------------------------------------------------------------------------

func TestHandleAddLink_InvalidDirection(t *testing.T) {
	svc, _, sessMgr, _ := setupEditorService(t)
	addEditorSession(t, sessMgr, "u1", "r1")

	evt, err := svc.handleAddLink("u1", &gamev1.AddLinkRequest{FromRoomId: "r1", Direction: "diagonal", ToRoomId: "r2"})
	require.NoError(t, err)
	assert.Contains(t, evtText(evt), "invalid direction")
}

func TestHandleAddLink_Success(t *testing.T) {
	svc, worldMgr, sessMgr, _ := setupEditorService(t)
	addEditorSession(t, sessMgr, "u1", "r1")

	evt, err := svc.handleAddLink("u1", &gamev1.AddLinkRequest{FromRoomId: "r1", Direction: "north", ToRoomId: "r2"})
	require.NoError(t, err)
	assert.Contains(t, evtText(evt), "r1")

	r1, ok := worldMgr.GetRoom("r1")
	require.True(t, ok)
	found := false
	for _, exit := range r1.Exits {
		if exit.Direction == world.North && exit.TargetRoom == "r2" {
			found = true
		}
	}
	assert.True(t, found)
}

// ---------------------------------------------------------------------------
// handleRemoveLink tests
// ---------------------------------------------------------------------------

func TestHandleRemoveLink_NotPresent(t *testing.T) {
	svc, _, sessMgr, _ := setupEditorService(t)
	addEditorSession(t, sessMgr, "u1", "r1")

	evt, err := svc.handleRemoveLink("u1", &gamev1.RemoveLinkRequest{RoomId: "r1", Direction: "north"})
	require.NoError(t, err)
	assert.Contains(t, evtText(evt), "no north exit")
}

func TestHandleRemoveLink_Success(t *testing.T) {
	svc, worldMgr, sessMgr, _ := setupEditorService(t)
	addEditorSession(t, sessMgr, "u1", "r1")

	// First add a link.
	_, err := svc.handleAddLink("u1", &gamev1.AddLinkRequest{FromRoomId: "r1", Direction: "north", ToRoomId: "r2"})
	require.NoError(t, err)

	evt, err := svc.handleRemoveLink("u1", &gamev1.RemoveLinkRequest{RoomId: "r1", Direction: "north"})
	require.NoError(t, err)
	assert.Contains(t, evtText(evt), "Removed")

	r1, ok := worldMgr.GetRoom("r1")
	require.True(t, ok)
	for _, exit := range r1.Exits {
		assert.NotEqual(t, world.North, exit.Direction)
	}
}

// ---------------------------------------------------------------------------
// handleSetRoom tests
// ---------------------------------------------------------------------------

func TestHandleSetRoom_UnknownField(t *testing.T) {
	svc, _, sessMgr, _ := setupEditorService(t)
	addEditorSession(t, sessMgr, "u1", "r1")

	evt, err := svc.handleSetRoom("u1", &gamev1.SetRoomRequest{Field: "color", Value: "blue"})
	require.NoError(t, err)
	assert.Contains(t, evtText(evt), "unknown field")
}

func TestHandleSetRoom_InvalidDangerLevel(t *testing.T) {
	svc, _, sessMgr, _ := setupEditorService(t)
	addEditorSession(t, sessMgr, "u1", "r1")

	evt, err := svc.handleSetRoom("u1", &gamev1.SetRoomRequest{Field: "danger_level", Value: "lethal"})
	require.NoError(t, err)
	assert.Contains(t, evtText(evt), "invalid danger level")
}

func TestHandleSetRoom_Success(t *testing.T) {
	svc, worldMgr, sessMgr, _ := setupEditorService(t)
	addEditorSession(t, sessMgr, "u1", "r1")

	evt, err := svc.handleSetRoom("u1", &gamev1.SetRoomRequest{Field: "title", Value: "New Title"})
	require.NoError(t, err)
	assert.Contains(t, evtText(evt), "r1")

	room, ok := worldMgr.GetRoom("r1")
	require.True(t, ok)
	assert.Equal(t, "New Title", room.Title)
}

// ---------------------------------------------------------------------------
// handleEditorCmds tests
// ---------------------------------------------------------------------------

func TestHandleEditorCmds_ReturnsSortedList(t *testing.T) {
	svc, _, sessMgr, _ := setupEditorService(t)
	addEditorSession(t, sessMgr, "u1", "r1")

	evt, err := svc.handleEditorCmds("u1")
	require.NoError(t, err)
	require.NotNil(t, evt)

	text := evtText(evt)
	assert.Contains(t, text, "Editor commands:")
	assert.Contains(t, text, "ecmds")
	assert.Contains(t, text, "spawnnpc")
}

func TestHandleEditorCmds_DeniedPlayerRole(t *testing.T) {
	svc, _, sessMgr, _ := setupEditorService(t)
	addPlayerSession(t, sessMgr, "u1", "r1")

	evt, err := svc.handleEditorCmds("u1")
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evtText(evt), "permission denied")
}

func TestHandleEditorCmds_AllCategoryEditor(t *testing.T) {
	svc, _, sessMgr, _ := setupEditorService(t)
	addEditorSession(t, sessMgr, "u1", "r1")

	evt, err := svc.handleEditorCmds("u1")
	require.NoError(t, err)

	text := evtText(evt)
	// All handler constants registered as CategoryEditor should appear.
	for _, cmd := range command.BuiltinCommands() {
		if cmd.Category == command.CategoryEditor {
			assert.Contains(t, text, cmd.Name)
		}
	}
}
