package gameserver

// Tests for AdminListZones and AdminListRooms gRPC handlers.
// REQ-AUI-1: AdminListZones MUST return a summary for every zone in the world manager.
// REQ-AUI-2: AdminListZones MUST report an accurate RoomCount for each zone.
// REQ-AUI-3: AdminListRooms MUST return all rooms for a valid zone, and codes.NotFound for an unknown zone.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"pgregory.net/rapid"

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

// TestProperty_AdminListZones_NeverErrors is a property test verifying AdminListZones
// never returns an error for the standard world state. REQ-AUI-1.
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
	})
}
