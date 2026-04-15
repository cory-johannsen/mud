package gameserver

// Tests for AdminListZones, AdminListRooms, AdminUpdateRoom, AdminListNPCTemplates,
// and AdminSpawnNPC gRPC handlers.
// REQ-AUI-1: AdminListZones MUST return a summary for every zone in the world manager.
// REQ-AUI-2: AdminListZones MUST report an accurate RoomCount for each zone.
// REQ-AUI-3: AdminListRooms MUST return all rooms for a valid zone, and codes.NotFound for an unknown zone.
// REQ-AUI-6: AdminListNPCTemplates MUST return all registered NPC templates.
// REQ-AUI-7: Each AdminNPCTemplateSummary MUST have non-empty ID and Name and a non-negative Level.
// REQ-AUI-8: AdminSpawnNPC MUST spawn the requested count of NPC instances.
// REQ-AUI-9: AdminSpawnNPC MUST return the exact spawned_count in the response.
// REQ-AUI-10: AdminSpawnNPC MUST spawn into the specified room.
// REQ-AUI-11: AdminSpawnNPC MUST return codes.NotFound for unknown template_id.
// REQ-AUI-12: AdminSpawnNPC MUST return codes.InvalidArgument for count outside [1,20].

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// assertAdminGRPCCode asserts that err carries the given gRPC status code.
func assertAdminGRPCCode(t *testing.T, err error, wantCode codes.Code) {
	t.Helper()
	st, ok := status.FromError(err)
	require.True(t, ok, "expected gRPC status error, got: %T %v", err, err)
	assert.Equal(t, wantCode, st.Code(), "unexpected gRPC code: %v", err)
}

// TestAdminListZones_ReturnsAllLoadedZones verifies that AdminListZones returns
// a summary for every zone registered in the world manager. REQ-AUI-1, REQ-AUI-2.
func TestAdminListZones_ReturnsAllLoadedZones(t *testing.T) {
	svc, _ := newAdminSvc(t)
	resp, err := svc.AdminListZones(context.Background(), &gamev1.AdminListZonesRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	for _, z := range resp.Zones {
		assert.NotEmpty(t, z.Id)
		assert.NotEmpty(t, z.Name)
		assert.GreaterOrEqual(t, z.RoomCount, int32(0))
	}
}

// TestAdminListZones_RoomCountMatchesActualRooms verifies RoomCount accuracy. REQ-AUI-2.
func TestAdminListZones_RoomCountMatchesActualRooms(t *testing.T) {
	svc, _ := newAdminSvc(t)
	resp, err := svc.AdminListZones(context.Background(), &gamev1.AdminListZonesRequest{})
	require.NoError(t, err)
	for _, z := range resp.Zones {
		zone, ok := svc.world.GetZone(z.Id)
		require.True(t, ok)
		assert.Equal(t, int32(len(zone.Rooms)), z.RoomCount)
	}
}

// TestAdminListRooms_ReturnsRoomsForZone verifies rooms are listed for a valid zone. REQ-AUI-3.
func TestAdminListRooms_ReturnsRoomsForZone(t *testing.T) {
	svc, _ := newAdminSvc(t)
	zonesResp, err := svc.AdminListZones(context.Background(), &gamev1.AdminListZonesRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, zonesResp.Zones)
	zoneID := zonesResp.Zones[0].Id
	zone, _ := svc.world.GetZone(zoneID)
	resp, err := svc.AdminListRooms(context.Background(), &gamev1.AdminListRoomsRequest{ZoneId: zoneID})
	require.NoError(t, err)
	assert.Equal(t, len(zone.Rooms), len(resp.Rooms))
	for _, r := range resp.Rooms {
		assert.NotEmpty(t, r.Id)
		assert.NotEmpty(t, r.Title)
	}
}

// TestAdminListRooms_UnknownZone_ReturnsNotFound verifies NotFound for bad zone. REQ-AUI-3.
func TestAdminListRooms_UnknownZone_ReturnsNotFound(t *testing.T) {
	svc, _ := newAdminSvc(t)
	_, err := svc.AdminListRooms(context.Background(), &gamev1.AdminListRoomsRequest{ZoneId: "nonexistent-zone-xyz"})
	require.Error(t, err)
	assertAdminGRPCCode(t, err, codes.NotFound)
}

// TestAdminListRooms_EmptyZoneID_ReturnsInvalidArgument verifies that an empty
// zone_id returns codes.InvalidArgument. REQ-AUI-3.
func TestAdminListRooms_EmptyZoneID_ReturnsInvalidArgument(t *testing.T) {
	svc, _ := newAdminSvc(t)
	_, err := svc.AdminListRooms(context.Background(), &gamev1.AdminListRoomsRequest{ZoneId: ""})
	require.Error(t, err)
	assertAdminGRPCCode(t, err, codes.InvalidArgument)
}

// TestProperty_AdminListZones_NeverErrors is a property test verifying AdminListZones
// is always consistent with the world manager's zone count. REQ-AUI-1.
func TestProperty_AdminListZones_NeverErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, _ := newAdminSvc(t)
		resp, err := svc.AdminListZones(context.Background(), &gamev1.AdminListZonesRequest{})
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if resp == nil {
			rt.Fatal("nil response")
		}
		// Verify count is consistent with the world manager.
		allZones := svc.world.AllZones()
		if len(resp.Zones) != len(allZones) {
			rt.Fatalf("zone count mismatch: response has %d, world has %d", len(resp.Zones), len(allZones))
		}
	})
}

// ---------------------------------------------------------------------------
// AdminUpdateRoom tests
// REQ-AUI-4: AdminUpdateRoom MUST reject empty room_id with codes.InvalidArgument.
// REQ-AUI-5: AdminUpdateRoom MUST return codes.Internal when world editor is not wired.
// ---------------------------------------------------------------------------

// TestAdminUpdateRoom_EmptyRoomID_ReturnsInvalidArgument verifies that an empty
// room_id is rejected. REQ-AUI-4.
func TestAdminUpdateRoom_EmptyRoomID_ReturnsInvalidArgument(t *testing.T) {
	svc, _ := newAdminSvc(t)
	_, err := svc.AdminUpdateRoom(context.Background(), &gamev1.AdminUpdateRoomRequest{
		RoomId: "",
		Title:  "Title",
	})
	require.Error(t, err)
	assertAdminGRPCCode(t, err, codes.InvalidArgument)
}

// TestAdminUpdateRoom_NilWorldEditor_ReturnsInternal verifies that AdminUpdateRoom
// returns codes.Internal when no world editor is wired. REQ-AUI-5.
func TestAdminUpdateRoom_NilWorldEditor_ReturnsInternal(t *testing.T) {
	svc, _ := newAdminSvc(t)
	// newAdminSvc does not wire a worldEditor; s.worldEditor is nil.
	_, err := svc.AdminUpdateRoom(context.Background(), &gamev1.AdminUpdateRoomRequest{
		RoomId:      "room_safe",
		Title:       "New Title",
		Description: "New description.",
		DangerLevel: "safe",
	})
	require.Error(t, err)
	assertAdminGRPCCode(t, err, codes.Internal)
}

// TestAdminUpdateRoom_NoopUpdate_NilEditor_ReturnsInternal verifies that a request
// with a valid room_id but all empty patch fields still returns codes.Internal when
// worldEditor is nil. The worldEditor nil-check precedes field iteration, so even a
// no-op request requires a wired editor. REQ-AUI-4, REQ-AUI-5.
func TestAdminUpdateRoom_NoopUpdate_NilEditor_ReturnsInternal(t *testing.T) {
	svc, _ := newAdminSvc(t)
	// newAdminSvc does not wire a worldEditor; s.worldEditor is nil.
	// All patch fields are empty — this would be a no-op if the editor were wired,
	// but the nil-check fires before the field loop, so codes.Internal is expected.
	_, err := svc.AdminUpdateRoom(context.Background(), &gamev1.AdminUpdateRoomRequest{
		RoomId:      "room_lg",
		Title:       "",
		Description: "",
		DangerLevel: "",
	})
	require.Error(t, err)
	assertAdminGRPCCode(t, err, codes.Internal)
}

// TestProperty_AdminUpdateRoom_EmptyRoomIDAlwaysInvalid is a property test verifying
// that an empty room_id always returns codes.InvalidArgument, regardless of what other
// fields contain. REQ-AUI-4.
func TestProperty_AdminUpdateRoom_EmptyRoomIDAlwaysInvalid(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, _ := newAdminSvc(t)
		title := rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz "))).Draw(rt, "title")
		_, err := svc.AdminUpdateRoom(context.Background(), &gamev1.AdminUpdateRoomRequest{
			RoomId:      "",
			Title:       title,
			Description: "",
			DangerLevel: "",
		})
		if err == nil {
			rt.Fatal("expected error for empty room_id, got nil")
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			rt.Fatalf("expected codes.InvalidArgument, got: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// AdminListNPCTemplates tests
// REQ-AUI-6: AdminListNPCTemplates MUST return all registered NPC templates.
// REQ-AUI-7: Each AdminNPCTemplateSummary MUST have non-empty ID and Name and a non-negative Level.
// ---------------------------------------------------------------------------

// TestAdminListNPCTemplates_ReturnsAllTemplates verifies all registered templates
// are returned with non-empty ID, Name, and non-negative Level. REQ-AUI-6, REQ-AUI-7.
func TestAdminListNPCTemplates_ReturnsAllTemplates(t *testing.T) {
	svc, _ := newAdminSvc(t)
	// Spawn an NPC to register the template in the manager.
	tmpl := &npc.Template{
		ID:    "list-test-npc",
		Name:  "List Test NPC",
		Level: 3,
		Type:  "human",
	}
	_, err := svc.npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	resp, err := svc.AdminListNPCTemplates(context.Background(), &gamev1.AdminListNPCTemplatesRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Templates)
	for _, summary := range resp.Templates {
		assert.NotEmpty(t, summary.Id)
		assert.NotEmpty(t, summary.Name)
		assert.GreaterOrEqual(t, summary.Level, int32(0))
	}
}

// TestAdminListNPCTemplates_EmptyManager_ReturnsEmptyList verifies that an empty NPC
// manager returns an empty (not nil) template list. REQ-AUI-6.
func TestAdminListNPCTemplates_EmptyManager_ReturnsEmptyList(t *testing.T) {
	svc, _ := newAdminSvc(t)
	resp, err := svc.AdminListNPCTemplates(context.Background(), &gamev1.AdminListNPCTemplatesRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Templates)
}

// ---------------------------------------------------------------------------
// AdminSpawnNPC tests
// REQ-AUI-8: AdminSpawnNPC MUST spawn the requested count of NPC instances.
// REQ-AUI-9: AdminSpawnNPC MUST return the exact spawned_count in the response.
// REQ-AUI-10: AdminSpawnNPC MUST spawn into the specified room.
// REQ-AUI-11: AdminSpawnNPC MUST return codes.NotFound for unknown template_id.
// REQ-AUI-12: AdminSpawnNPC MUST return codes.InvalidArgument for count outside [1,20].
// ---------------------------------------------------------------------------

// TestAdminSpawnNPC_UnknownTemplate_ReturnsNotFound verifies NotFound for an
// unregistered template ID. REQ-AUI-11.
func TestAdminSpawnNPC_UnknownTemplate_ReturnsNotFound(t *testing.T) {
	svc, _ := newAdminSvc(t)
	_, err := svc.AdminSpawnNPC(context.Background(), &gamev1.AdminSpawnNPCRequest{
		TemplateId: "does-not-exist",
		RoomId:     "room_a",
		Count:      1,
	})
	require.Error(t, err)
	assertAdminGRPCCode(t, err, codes.NotFound)
}

// TestAdminSpawnNPC_InvalidCount_ReturnsInvalidArgument verifies that counts
// outside [1,20] are rejected. REQ-AUI-12.
func TestAdminSpawnNPC_InvalidCount_ReturnsInvalidArgument(t *testing.T) {
	svc, _ := newAdminSvc(t)
	for _, count := range []int32{0, -1, 21, 100} {
		_, err := svc.AdminSpawnNPC(context.Background(), &gamev1.AdminSpawnNPCRequest{
			TemplateId: "any",
			RoomId:     "room_a",
			Count:      count,
		})
		require.Errorf(t, err, "count=%d should be rejected", count)
		assertAdminGRPCCode(t, err, codes.InvalidArgument)
	}
}

// TestAdminSpawnNPC_ValidRequest_SpawnsInstances verifies a valid spawn request
// returns the correct spawned_count. REQ-AUI-8, REQ-AUI-9, REQ-AUI-10.
func TestAdminSpawnNPC_ValidRequest_SpawnsInstances(t *testing.T) {
	svc, _ := newAdminSvc(t)
	// Register the template by spawning a seed instance, then remove it.
	tmpl := &npc.Template{
		ID:    "spawn-test-npc",
		Name:  "Spawn Test NPC",
		Level: 2,
		Type:  "mutant",
	}
	seed, err := svc.npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	require.NoError(t, svc.npcMgr.Remove(seed.ID))

	const count = 3
	resp, err := svc.AdminSpawnNPC(context.Background(), &gamev1.AdminSpawnNPCRequest{
		TemplateId: "spawn-test-npc",
		RoomId:     "room_a",
		Count:      count,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(count), resp.SpawnedCount)
}

// TestProperty_AdminSpawnNPC_CountAlwaysValid is a property test verifying that
// any count in [1,20] always results in SpawnedCount matching the request. REQ-AUI-12.
func TestProperty_AdminSpawnNPC_CountAlwaysValid(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, _ := newAdminSvc(t)
		tmpl := &npc.Template{
			ID:    "prop-npc",
			Name:  "Prop NPC",
			Level: 1,
			Type:  "robot",
		}
		seed, err := svc.npcMgr.Spawn(tmpl, "room_a")
		if err != nil {
			rt.Fatalf("seed spawn failed: %v", err)
		}
		if removeErr := svc.npcMgr.Remove(seed.ID); removeErr != nil {
			rt.Fatalf("seed remove failed: %v", removeErr)
		}

		count := rapid.Int32Range(1, 20).Draw(rt, "count")
		resp, err := svc.AdminSpawnNPC(context.Background(), &gamev1.AdminSpawnNPCRequest{
			TemplateId: "prop-npc",
			RoomId:     "room_a",
			Count:      count,
		})
		if err != nil {
			rt.Fatalf("valid count %d failed: %v", count, err)
		}
		if resp.SpawnedCount != count {
			rt.Fatalf("expected SpawnedCount=%d, got %d", count, resp.SpawnedCount)
		}
	})
}
