package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cory-johannsen/mud/cmd/webclient/eventbus"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ManagedSession is the subset of a live player session that the admin API requires.
//
// Precondition: All methods are safe for concurrent use.
type ManagedSession interface {
	CharID() int64
	AccountID() int64
	PlayerName() string
	Level() int
	RoomID() string
	Zone() string
	CurrentHP() int
	SendAdminMessage(text string) error
	Kick() error
}

// SessionManager enumerates and retrieves live player sessions.
type SessionManager interface {
	AllSessions() ([]ManagedSession, error)
	GetSession(charID int64) (ManagedSession, bool)
	TeleportPlayer(charID int64, roomID string) error
}

// AdminAccountStore is the subset of AccountRepository required by the admin handler.
type AdminAccountStore interface {
	SearchByUsernamePrefix(ctx context.Context, prefix string, limit int) ([]postgres.Account, error)
	GetByID(ctx context.Context, id int64) (postgres.Account, error)
	UpdateRoleAndBanned(ctx context.Context, id int64, role string, banned bool) error
}

// ZoneSummary is a lightweight zone descriptor for the admin API.
type ZoneSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DangerLevel string `json:"danger_level"`
	RoomCount   int    `json:"room_count"`
}

// RoomSummary is a lightweight room descriptor for the admin API.
type RoomSummary struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	DangerLevel string `json:"danger_level"`
}

// RoomPatch holds mutable fields that an admin may update on a room.
type RoomPatch struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	DangerLevel string `json:"danger_level"`
}

// NPCTemplate is a lightweight NPC template descriptor for the admin API.
type NPCTemplate struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
	Type  string `json:"type"`
}

// WorldEditor provides read/write access to the game world for admin operations.
type WorldEditor interface {
	AllZones() []ZoneSummary
	RoomsInZone(zoneID string) ([]RoomSummary, error)
	UpdateRoom(roomID string, patch RoomPatch) error
	AllNPCTemplates() []NPCTemplate
}

// AdminHandler implements all /api/admin/* endpoints.
//
// Invariant: sessions, accounts, world, and bus must all be non-nil.
type AdminHandler struct {
	sessions SessionManager
	accounts AdminAccountStore
	world    WorldEditor
	bus      *eventbus.EventBus
}

// NewAdminHandler creates an AdminHandler.
//
// Precondition: all arguments must be non-nil.
func NewAdminHandler(sessions SessionManager, accounts AdminAccountStore, world WorldEditor, bus *eventbus.EventBus) *AdminHandler {
	return &AdminHandler{
		sessions: sessions,
		accounts: accounts,
		world:    world,
		bus:      bus,
	}
}

// playerResponse is the JSON shape for an online player.
type playerResponse struct {
	CharID    int64  `json:"char_id"`
	AccountID int64  `json:"account_id"`
	Name      string `json:"name"`
	Level     int    `json:"level"`
	RoomID    string `json:"room_id"`
	Zone      string `json:"zone"`
	CurrentHP int    `json:"current_hp"`
}

// HandleListPlayers handles GET /api/admin/players.
//
// Postcondition: Returns JSON array of playerResponse for all online players.
// Postcondition: Returns HTTP 502 with {"error":"gameserver unavailable"} on RPC error.
func (ah *AdminHandler) HandleListPlayers(w http.ResponseWriter, r *http.Request) {
	sessions, err := ah.sessions.AllSessions()
	if err != nil {
		writeError(w, http.StatusBadGateway, "gameserver unavailable")
		return
	}
	resp := make([]playerResponse, 0, len(sessions))
	for _, s := range sessions {
		resp = append(resp, playerResponse{
			CharID:    s.CharID(),
			AccountID: s.AccountID(),
			Name:      s.PlayerName(),
			Level:     s.Level(),
			RoomID:    s.RoomID(),
			Zone:      s.Zone(),
			CurrentHP: s.CurrentHP(),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleKickPlayer handles POST /api/admin/players/:char_id/kick.
//
// Postcondition: Returns 404 if session not found; 200 on success.
func (ah *AdminHandler) HandleKickPlayer(w http.ResponseWriter, r *http.Request) {
	charID, ok := parseCharID(w, r)
	if !ok {
		return
	}
	sess, found := ah.sessions.GetSession(charID)
	if !found {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if err := sess.Kick(); err != nil {
		writeError(w, http.StatusInternalServerError, "kick failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "kicked"})
}

// HandleMessagePlayer handles POST /api/admin/players/:char_id/message.
//
// Precondition: Request body MUST contain {"text":"non-empty"}.
// Postcondition: Returns 400 if text is empty; 404 if session not found; 200 on success.
func (ah *AdminHandler) HandleMessagePlayer(w http.ResponseWriter, r *http.Request) {
	charID, ok := parseCharID(w, r)
	if !ok {
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		writeError(w, http.StatusBadRequest, "text must not be empty")
		return
	}
	sess, found := ah.sessions.GetSession(charID)
	if !found {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if err := sess.SendAdminMessage(body.Text); err != nil {
		writeError(w, http.StatusInternalServerError, "send message failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// HandleTeleportPlayer handles POST /api/admin/players/:char_id/teleport.
//
// Precondition: Request body MUST contain {"room_id":"non-empty"}.
// Postcondition: Returns 400 if room_id is empty; 404 if session not found; 200 on success.
func (ah *AdminHandler) HandleTeleportPlayer(w http.ResponseWriter, r *http.Request) {
	charID, ok := parseCharID(w, r)
	if !ok {
		return
	}
	var body struct {
		RoomID string `json:"room_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(body.RoomID) == "" {
		writeError(w, http.StatusBadRequest, "room_id must not be empty")
		return
	}
	if err := ah.sessions.TeleportPlayer(charID, body.RoomID); err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusBadGateway, "gameserver unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// accountResponse is the JSON shape for an account in admin search results.
type accountResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Banned   bool   `json:"banned"`
}

// HandleSearchAccounts handles GET /api/admin/accounts?q=<prefix>.
//
// Postcondition: Returns up to 100 accounts matching the prefix (all if q is empty).
func (ah *AdminHandler) HandleSearchAccounts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	accounts, err := ah.accounts.SearchByUsernamePrefix(r.Context(), q, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}
	resp := make([]accountResponse, 0, len(accounts))
	for _, a := range accounts {
		resp = append(resp, accountResponse{
			ID:       a.ID,
			Username: a.Username,
			Role:     a.Role,
			Banned:   a.Banned,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleUpdateAccount handles PUT /api/admin/accounts/:id.
//
// Precondition: Request body MUST contain {"role":"<valid>","banned":<bool>}.
// Postcondition: Returns 400 on invalid role; 404 if account not found; 200 on success.
func (ah *AdminHandler) HandleUpdateAccount(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}
	var body struct {
		Role   string `json:"role"`
		Banned bool   `json:"banned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if !postgres.ValidRole(body.Role) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid role %q", body.Role))
		return
	}
	// Verify account exists.
	if _, err := ah.accounts.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, postgres.ErrAccountNotFound) {
			writeError(w, http.StatusNotFound, "account not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup failed")
		return
	}
	if err := ah.accounts.UpdateRoleAndBanned(r.Context(), id, body.Role, body.Banned); err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// HandleListZones handles GET /api/admin/zones.
//
// Postcondition: Returns JSON array of ZoneSummary.
func (ah *AdminHandler) HandleListZones(w http.ResponseWriter, r *http.Request) {
	zones := ah.world.AllZones()
	writeJSON(w, http.StatusOK, zones)
}

// HandleListRooms handles GET /api/admin/zones/:zone_id/rooms.
//
// Postcondition: Returns 404 if zone not found; JSON array of RoomSummary otherwise.
func (ah *AdminHandler) HandleListRooms(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zone_id")
	rooms, err := ah.world.RoomsInZone(zoneID)
	if err != nil {
		writeError(w, http.StatusNotFound, "zone not found")
		return
	}
	writeJSON(w, http.StatusOK, rooms)
}

// HandleUpdateRoom handles PUT /api/admin/rooms/:room_id.
//
// Precondition: Request body MUST contain a valid RoomPatch with non-empty Title.
// Postcondition: Returns 400 on validation failure; 200 on success.
func (ah *AdminHandler) HandleUpdateRoom(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("room_id")
	var patch RoomPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(patch.Title) == "" {
		writeError(w, http.StatusBadRequest, "title must not be empty")
		return
	}
	if err := ah.world.UpdateRoom(roomID, patch); err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// HandleListNPCs handles GET /api/admin/npcs.
//
// Postcondition: Returns JSON array of NPCTemplate.
func (ah *AdminHandler) HandleListNPCs(w http.ResponseWriter, r *http.Request) {
	templates := ah.world.AllNPCTemplates()
	if templates == nil {
		templates = []NPCTemplate{}
	}
	writeJSON(w, http.StatusOK, templates)
}

// HandleSpawnNPC handles POST /api/admin/rooms/:room_id/spawn-npc.
//
// Precondition: Request body MUST contain {"npc_id":"non-empty","count":>=1}.
// Postcondition: Returns 400 on validation failure; 200 on success.
func (ah *AdminHandler) HandleSpawnNPC(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NPCID  string `json:"npc_id"`
		Count  int    `json:"count"`
		RoomID string `json:"room_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(body.NPCID) == "" {
		writeError(w, http.StatusBadRequest, "npc_id must not be empty")
		return
	}
	if body.Count < 1 {
		writeError(w, http.StatusBadRequest, "count must be >= 1")
		return
	}
	// Spawn via game server is not yet wired through an admin gRPC stream (REQ-WC-38).
	writeError(w, http.StatusNotImplemented, "not implemented")
	return
}

// HandleAdminEvents handles GET /api/admin/events — an SSE stream of game events.
// The JWT may be supplied via Authorization header or the ?token= query parameter,
// since EventSource in the browser does not support custom headers.
//
// Precondition: w MUST implement http.Flusher.
// Postcondition: Streams Server-Sent Events until the client disconnects.
func (ah *AdminHandler) HandleAdminEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	// Parse ?types= filter.
	typeFilter := make(map[string]bool)
	if raw := r.URL.Query().Get("types"); raw != "" {
		for _, t := range strings.Split(raw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				typeFilter[t] = true
			}
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, unsub := ah.bus.Subscribe()
	defer unsub()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			if len(typeFilter) > 0 && !typeFilter[ev.Type] {
				continue
			}
			payload, err := json.Marshal(struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
				Time    time.Time       `json:"time"`
			}{
				Type:    ev.Type,
				Payload: ev.Payload,
				Time:    ev.Time,
			})
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}

// parseCharID extracts and validates the char_id path value.
func parseCharID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.PathValue("char_id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid char_id")
		return 0, false
	}
	return id, true
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
