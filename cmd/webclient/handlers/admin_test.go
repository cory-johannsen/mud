package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/cmd/webclient/eventbus"
	"github.com/cory-johannsen/mud/cmd/webclient/handlers"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// ---- Stub implementations ----

// stubManagedSession implements handlers.ManagedSession.
type stubManagedSession struct {
	charID    int64
	accountID int64
	name      string
	level     int
	roomID    string
	zone      string
	hp        int
	kicked    bool
	msgSent   string
}

func (s *stubManagedSession) CharID() int64      { return s.charID }
func (s *stubManagedSession) AccountID() int64   { return s.accountID }
func (s *stubManagedSession) PlayerName() string { return s.name }
func (s *stubManagedSession) Level() int         { return s.level }
func (s *stubManagedSession) RoomID() string     { return s.roomID }
func (s *stubManagedSession) Zone() string       { return s.zone }
func (s *stubManagedSession) CurrentHP() int     { return s.hp }
func (s *stubManagedSession) SendAdminMessage(text string) error {
	s.msgSent = text
	return nil
}
func (s *stubManagedSession) Kick() error {
	s.kicked = true
	return nil
}

// stubSessionManager implements handlers.SessionManager.
type stubSessionManager struct {
	sessions []*stubManagedSession
}

func (m *stubSessionManager) AllSessions() ([]handlers.ManagedSession, error) {
	result := make([]handlers.ManagedSession, len(m.sessions))
	for i, s := range m.sessions {
		result[i] = s
	}
	return result, nil
}

func (m *stubSessionManager) GetSession(charID int64) (handlers.ManagedSession, bool) {
	for _, s := range m.sessions {
		if s.charID == charID {
			return s, true
		}
	}
	return nil, false
}

func (m *stubSessionManager) TeleportPlayer(_ int64, _ string) error { return nil }

// stubAccountStore implements handlers.AdminAccountStore.
type stubAccountStore struct {
	accounts []postgres.Account
	updateID int64
}

func (s *stubAccountStore) SearchByUsernamePrefix(ctx context.Context, prefix string, limit int) ([]postgres.Account, error) {
	if prefix == "" {
		return s.accounts, nil
	}
	var results []postgres.Account
	for _, a := range s.accounts {
		if strings.HasPrefix(a.Username, prefix) {
			results = append(results, a)
		}
	}
	return results, nil
}

func (s *stubAccountStore) GetByID(ctx context.Context, id int64) (postgres.Account, error) {
	for _, a := range s.accounts {
		if a.ID == id {
			return a, nil
		}
	}
	return postgres.Account{}, postgres.ErrAccountNotFound
}

func (s *stubAccountStore) UpdateRoleAndBanned(ctx context.Context, id int64, role string, banned bool) error {
	s.updateID = id
	return nil
}

// stubWorldEditor implements handlers.WorldEditor.
type stubWorldEditor struct {
	zones    []handlers.ZoneSummary
	rooms    map[string][]handlers.RoomSummary
	roomsErr map[string]error
	updated  *handlers.RoomPatch
}

func (w *stubWorldEditor) AllZones() []handlers.ZoneSummary { return w.zones }
func (w *stubWorldEditor) RoomsInZone(zoneID string) ([]handlers.RoomSummary, error) {
	if err, ok := w.roomsErr[zoneID]; ok {
		return nil, err
	}
	return w.rooms[zoneID], nil
}
func (w *stubWorldEditor) UpdateRoom(roomID string, patch handlers.RoomPatch) error {
	w.updated = &patch
	return nil
}
func (w *stubWorldEditor) AllNPCTemplates() []handlers.NPCTemplate { return nil }

// ---- Test helpers ----

func makeAdminHandler(sessions *stubSessionManager, accounts *stubAccountStore, world *stubWorldEditor, bus *eventbus.EventBus) *handlers.AdminHandler {
	return handlers.NewAdminHandler(sessions, accounts, world, bus)
}

// ---- Task 3 Tests: Players API ----

func TestHandleListPlayers_Empty(t *testing.T) {
	bus := eventbus.New(16)
	sm := &stubSessionManager{}
	h := makeAdminHandler(sm, &stubAccountStore{}, &stubWorldEditor{}, bus)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/players", nil)
	w := httptest.NewRecorder()
	h.HandleListPlayers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if len(resp) != 0 {
		t.Fatalf("expected empty array, got %d items", len(resp))
	}
}

func TestHandleListPlayers_MultipleSessions(t *testing.T) {
	bus := eventbus.New(16)
	sm := &stubSessionManager{
		sessions: []*stubManagedSession{
			{charID: 1, accountID: 10, name: "Alice", level: 5, roomID: "room-1", zone: "dungeon", hp: 30},
			{charID: 2, accountID: 20, name: "Bob", level: 3, roomID: "room-2", zone: "town", hp: 20},
			{charID: 3, accountID: 30, name: "Carol", level: 7, roomID: "room-3", zone: "wild", hp: 50},
		},
	}
	h := makeAdminHandler(sm, &stubAccountStore{}, &stubWorldEditor{}, bus)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/players", nil)
	w := httptest.NewRecorder()
	h.HandleListPlayers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if len(resp) != 3 {
		t.Fatalf("expected 3 players, got %d", len(resp))
	}
	// Verify JSON shape contains expected fields.
	for _, p := range resp {
		for _, field := range []string{"char_id", "account_id", "name", "level", "room_id", "zone", "current_hp"} {
			if _, ok := p[field]; !ok {
				t.Errorf("missing field %q in player response", field)
			}
		}
	}
}

func TestHandleKickPlayer_NotFound(t *testing.T) {
	bus := eventbus.New(16)
	sm := &stubSessionManager{}
	h := makeAdminHandler(sm, &stubAccountStore{}, &stubWorldEditor{}, bus)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/players/999/kick", nil)
	req.SetPathValue("char_id", "999")
	w := httptest.NewRecorder()
	h.HandleKickPlayer(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleKickPlayer_Success(t *testing.T) {
	bus := eventbus.New(16)
	sess := &stubManagedSession{charID: 42, name: "Alice"}
	sm := &stubSessionManager{sessions: []*stubManagedSession{sess}}
	h := makeAdminHandler(sm, &stubAccountStore{}, &stubWorldEditor{}, bus)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/players/42/kick", nil)
	req.SetPathValue("char_id", "42")
	w := httptest.NewRecorder()
	h.HandleKickPlayer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !sess.kicked {
		t.Fatal("expected Kick() to have been called on session")
	}
}

func TestHandleMessagePlayer_EmptyText(t *testing.T) {
	bus := eventbus.New(16)
	sess := &stubManagedSession{charID: 42, name: "Alice"}
	sm := &stubSessionManager{sessions: []*stubManagedSession{sess}}
	h := makeAdminHandler(sm, &stubAccountStore{}, &stubWorldEditor{}, bus)

	body := `{"text":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/players/42/message", strings.NewReader(body))
	req.SetPathValue("char_id", "42")
	w := httptest.NewRecorder()
	h.HandleMessagePlayer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleMessagePlayer_NotFound(t *testing.T) {
	bus := eventbus.New(16)
	sm := &stubSessionManager{}
	h := makeAdminHandler(sm, &stubAccountStore{}, &stubWorldEditor{}, bus)

	body := `{"text":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/players/999/message", strings.NewReader(body))
	req.SetPathValue("char_id", "999")
	w := httptest.NewRecorder()
	h.HandleMessagePlayer(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleMessagePlayer_Success(t *testing.T) {
	bus := eventbus.New(16)
	sess := &stubManagedSession{charID: 42, name: "Alice"}
	sm := &stubSessionManager{sessions: []*stubManagedSession{sess}}
	h := makeAdminHandler(sm, &stubAccountStore{}, &stubWorldEditor{}, bus)

	body := `{"text":"hello world"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/players/42/message", strings.NewReader(body))
	req.SetPathValue("char_id", "42")
	w := httptest.NewRecorder()
	h.HandleMessagePlayer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if sess.msgSent != "hello world" {
		t.Fatalf("expected message 'hello world', got %q", sess.msgSent)
	}
}

func TestHandleTeleportPlayer_NotFound(t *testing.T) {
	bus := eventbus.New(16)
	sm := &stubSessionManager{}
	h := makeAdminHandler(sm, &stubAccountStore{}, &stubWorldEditor{}, bus)

	body := `{"room_id":"zone:room"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/players/999/teleport", strings.NewReader(body))
	req.SetPathValue("char_id", "999")
	w := httptest.NewRecorder()
	h.HandleTeleportPlayer(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleTeleportPlayer_MissingRoomID(t *testing.T) {
	bus := eventbus.New(16)
	sess := &stubManagedSession{charID: 42}
	sm := &stubSessionManager{sessions: []*stubManagedSession{sess}}
	h := makeAdminHandler(sm, &stubAccountStore{}, &stubWorldEditor{}, bus)

	body := `{"room_id":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/players/42/teleport", strings.NewReader(body))
	req.SetPathValue("char_id", "42")
	w := httptest.NewRecorder()
	h.HandleTeleportPlayer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ---- Task 4 Tests: Accounts API ----

func TestHandleSearchAccounts_Empty(t *testing.T) {
	bus := eventbus.New(16)
	store := &stubAccountStore{}
	h := makeAdminHandler(&stubSessionManager{}, store, &stubWorldEditor{}, bus)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/accounts", nil)
	w := httptest.NewRecorder()
	h.HandleSearchAccounts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse body: %v", err)
	}
}

func TestHandleSearchAccounts_WithResults(t *testing.T) {
	bus := eventbus.New(16)
	store := &stubAccountStore{
		accounts: []postgres.Account{
			{ID: 1, Username: "admin", Role: "admin"},
			{ID: 2, Username: "alice", Role: "player"},
		},
	}
	h := makeAdminHandler(&stubSessionManager{}, store, &stubWorldEditor{}, bus)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/accounts?q=a", nil)
	w := httptest.NewRecorder()
	h.HandleSearchAccounts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(resp))
	}
}

func TestHandleUpdateAccount_InvalidRole(t *testing.T) {
	bus := eventbus.New(16)
	store := &stubAccountStore{
		accounts: []postgres.Account{{ID: 1, Username: "user1", Role: "player"}},
	}
	h := makeAdminHandler(&stubSessionManager{}, store, &stubWorldEditor{}, bus)

	body := `{"role":"superuser","banned":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/admin/accounts/1", strings.NewReader(body))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.HandleUpdateAccount(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleUpdateAccount_NotFound(t *testing.T) {
	bus := eventbus.New(16)
	store := &stubAccountStore{}
	h := makeAdminHandler(&stubSessionManager{}, store, &stubWorldEditor{}, bus)

	body := `{"role":"player","banned":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/admin/accounts/999", strings.NewReader(body))
	req.SetPathValue("id", "999")
	w := httptest.NewRecorder()
	h.HandleUpdateAccount(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleUpdateAccount_Success(t *testing.T) {
	bus := eventbus.New(16)
	store := &stubAccountStore{
		accounts: []postgres.Account{{ID: 1, Username: "alice", Role: "player"}},
	}
	h := makeAdminHandler(&stubSessionManager{}, store, &stubWorldEditor{}, bus)

	body := `{"role":"moderator","banned":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/admin/accounts/1", strings.NewReader(body))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.HandleUpdateAccount(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if store.updateID != 1 {
		t.Fatalf("expected updateID=1, got %d", store.updateID)
	}
}

// ---- Task 5 Tests: Zone/Room Editor ----

func TestHandleListZones(t *testing.T) {
	bus := eventbus.New(16)
	world := &stubWorldEditor{
		zones: []handlers.ZoneSummary{
			{ID: "zone-1", Name: "Dungeon", DangerLevel: "dangerous", RoomCount: 5},
		},
	}
	h := makeAdminHandler(&stubSessionManager{}, &stubAccountStore{}, world, bus)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/zones", nil)
	w := httptest.NewRecorder()
	h.HandleListZones(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 zone, got %d", len(resp))
	}
}

func TestHandleListRooms_Success(t *testing.T) {
	bus := eventbus.New(16)
	world := &stubWorldEditor{
		zones: []handlers.ZoneSummary{{ID: "zone-1", Name: "Dungeon"}},
		rooms: map[string][]handlers.RoomSummary{
			"zone-1": {
				{ID: "room-1", Title: "Entry", DangerLevel: "risky"},
				{ID: "room-2", Title: "Boss", DangerLevel: "deadly"},
			},
		},
	}
	h := makeAdminHandler(&stubSessionManager{}, &stubAccountStore{}, world, bus)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/zones/zone-1/rooms", nil)
	req.SetPathValue("zone_id", "zone-1")
	w := httptest.NewRecorder()
	h.HandleListRooms(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(resp))
	}
}

func TestHandleListRooms_ZoneNotFound(t *testing.T) {
	bus := eventbus.New(16)
	world := &stubWorldEditor{
		roomsErr: map[string]error{
			"missing": errors.New("zone not found"),
		},
	}
	h := makeAdminHandler(&stubSessionManager{}, &stubAccountStore{}, world, bus)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/zones/missing/rooms", nil)
	req.SetPathValue("zone_id", "missing")
	w := httptest.NewRecorder()
	h.HandleListRooms(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleUpdateRoom_MissingTitle(t *testing.T) {
	bus := eventbus.New(16)
	world := &stubWorldEditor{}
	h := makeAdminHandler(&stubSessionManager{}, &stubAccountStore{}, world, bus)

	body := `{"title":"","description":"desc","danger_level":"safe"}`
	req := httptest.NewRequest(http.MethodPut, "/api/admin/rooms/room-1", strings.NewReader(body))
	req.SetPathValue("room_id", "room-1")
	w := httptest.NewRecorder()
	h.HandleUpdateRoom(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleUpdateRoom_Success(t *testing.T) {
	bus := eventbus.New(16)
	world := &stubWorldEditor{}
	h := makeAdminHandler(&stubSessionManager{}, &stubAccountStore{}, world, bus)

	body := `{"title":"New Title","description":"Updated","danger_level":"risky"}`
	req := httptest.NewRequest(http.MethodPut, "/api/admin/rooms/room-1", strings.NewReader(body))
	req.SetPathValue("room_id", "room-1")
	w := httptest.NewRecorder()
	h.HandleUpdateRoom(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if world.updated == nil {
		t.Fatal("expected UpdateRoom to have been called")
	}
	if world.updated.Title != "New Title" {
		t.Fatalf("expected title 'New Title', got %q", world.updated.Title)
	}
}

// ---- Task 6 Tests: NPC Spawner ----

func TestHandleListNPCs(t *testing.T) {
	bus := eventbus.New(16)
	world := &stubWorldEditor{}
	h := makeAdminHandler(&stubSessionManager{}, &stubAccountStore{}, world, bus)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/npcs", nil)
	w := httptest.NewRecorder()
	h.HandleListNPCs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse body: %v", err)
	}
}

func TestHandleSpawnNPC_MissingFields(t *testing.T) {
	bus := eventbus.New(16)
	h := makeAdminHandler(&stubSessionManager{}, &stubAccountStore{}, &stubWorldEditor{}, bus)

	body := `{"npc_id":"","count":1,"room_id":"room-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/rooms/room-1/spawn-npc", strings.NewReader(body))
	req.SetPathValue("room_id", "room-1")
	w := httptest.NewRecorder()
	h.HandleSpawnNPC(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSpawnNPC_CountZero(t *testing.T) {
	bus := eventbus.New(16)
	h := makeAdminHandler(&stubSessionManager{}, &stubAccountStore{}, &stubWorldEditor{}, bus)

	body := `{"npc_id":"goblin","count":0,"room_id":"room-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/rooms/room-1/spawn-npc", strings.NewReader(body))
	req.SetPathValue("room_id", "room-1")
	w := httptest.NewRecorder()
	h.HandleSpawnNPC(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ---- Task 7 Tests: SSE Events endpoint ----

func TestHandleAdminEvents_FiltersTypes(t *testing.T) {
	bus := eventbus.New(64)
	h := makeAdminHandler(&stubSessionManager{}, &stubAccountStore{}, &stubWorldEditor{}, bus)

	// Set up an httptest server with a context we can cancel.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/admin/events?types=CombatEvent", nil).WithContext(ctx)
	w := newFlushRecorder()

	// Run handler in background.
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.HandleAdminEvents(w, req)
	}()

	// Publish events.
	time.Sleep(10 * time.Millisecond) // let handler subscribe
	bus.Publish(eventbus.Event{Type: "CombatEvent", Payload: json.RawMessage(`{"a":1}`), Time: time.Now()})
	bus.Publish(eventbus.Event{Type: "MessageEvent", Payload: json.RawMessage(`{"b":2}`), Time: time.Now()})
	bus.Publish(eventbus.Event{Type: "CombatEvent", Payload: json.RawMessage(`{"c":3}`), Time: time.Now()})

	// Give time to process.
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	body := w.body.String()
	combatCount := strings.Count(body, "CombatEvent")
	messageCount := strings.Count(body, "MessageEvent")
	if combatCount != 2 {
		t.Fatalf("expected 2 CombatEvent, got %d in body:\n%s", combatCount, body)
	}
	if messageCount != 0 {
		t.Fatalf("expected 0 MessageEvent, got %d", messageCount)
	}
}

func TestHandleAdminEvents_AllTypesWhenNoFilter(t *testing.T) {
	bus := eventbus.New(64)
	h := makeAdminHandler(&stubSessionManager{}, &stubAccountStore{}, &stubWorldEditor{}, bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/admin/events", nil).WithContext(ctx)
	w := newFlushRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.HandleAdminEvents(w, req)
	}()

	time.Sleep(10 * time.Millisecond)
	bus.Publish(eventbus.Event{Type: "CombatEvent", Payload: json.RawMessage(`{}`), Time: time.Now()})
	bus.Publish(eventbus.Event{Type: "MessageEvent", Payload: json.RawMessage(`{}`), Time: time.Now()})
	bus.Publish(eventbus.Event{Type: "RoomEvent", Payload: json.RawMessage(`{}`), Time: time.Now()})
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	body := w.body.String()
	if strings.Count(body, "data:") < 3 {
		t.Fatalf("expected at least 3 SSE data lines, got:\n%s", body)
	}
}

func TestHandleAdminEvents_DisconnectCleansUp(t *testing.T) {
	bus := eventbus.New(64)
	h := makeAdminHandler(&stubSessionManager{}, &stubAccountStore{}, &stubWorldEditor{}, bus)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/admin/events", nil).WithContext(ctx)
	w := newFlushRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.HandleAdminEvents(w, req)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel() // disconnect

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit after context cancel")
	}
}

// flushRecorder is an httptest.ResponseRecorder that also implements http.Flusher.
type flushRecorder struct {
	*httptest.ResponseRecorder
	body *strings.Builder
}

func newFlushRecorder() *flushRecorder {
	var sb strings.Builder
	return &flushRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		body:             &sb,
	}
}

func (f *flushRecorder) Write(b []byte) (int, error) {
	f.body.Write(b)
	return f.ResponseRecorder.Write(b)
}

func (f *flushRecorder) Flush() {
	f.ResponseRecorder.Flush()
}

// Ensure flushRecorder implements http.Flusher.
var _ http.Flusher = (*flushRecorder)(nil)

// stubBannedAccountStore implements AdminAccountStore with banned check.
type stubBannedAccountStore struct {
	stubAccountStore
}

// Ensure interface is satisfied.
var _ fmt.Stringer = (*stubManagedSession)(nil)

func (s *stubManagedSession) String() string {
	return fmt.Sprintf("Session{charID:%d, name:%q}", s.charID, s.name)
}
