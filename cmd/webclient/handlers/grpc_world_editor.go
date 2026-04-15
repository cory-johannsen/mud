package handlers

import (
	"context"
	"fmt"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap"
)

// grpcWorldEditor implements WorldEditor via the gameserver admin gRPC RPCs.
//
// Precondition: client must be non-nil.
// Postcondition: Each method makes one unary gRPC call and maps the result to the
// handlers layer types; gRPC errors are propagated as-is.
//
// Note: All gRPC calls use context.Background() because the WorldEditor interface
// does not thread request contexts. Admin operations are low-frequency and
// operator-facing, making this a known acceptable trade-off.
type grpcWorldEditor struct {
	client gamev1.GameServiceClient
	logger *zap.Logger
}

// NewGRPCWorldEditor returns a WorldEditor backed by the gameserver admin gRPC RPCs.
func NewGRPCWorldEditor(client gamev1.GameServiceClient, logger *zap.Logger) WorldEditor {
	return &grpcWorldEditor{client: client, logger: logger}
}

// AllZones calls AdminListZones and maps the result to []ZoneSummary.
func (g *grpcWorldEditor) AllZones() []ZoneSummary {
	resp, err := g.client.AdminListZones(context.Background(), &gamev1.AdminListZonesRequest{})
	if err != nil {
		g.logger.Error("AdminListZones failed", zap.Error(err))
		return []ZoneSummary{}
	}
	out := make([]ZoneSummary, 0, len(resp.Zones))
	for _, z := range resp.Zones {
		out = append(out, ZoneSummary{
			ID:          z.Id,
			Name:        z.Name,
			DangerLevel: z.DangerLevel,
			RoomCount:   int(z.RoomCount),
		})
	}
	return out
}

// RoomsInZone calls AdminListRooms for the given zone and maps the result to []RoomSummary.
func (g *grpcWorldEditor) RoomsInZone(zoneID string) ([]RoomSummary, error) {
	resp, err := g.client.AdminListRooms(context.Background(), &gamev1.AdminListRoomsRequest{ZoneId: zoneID})
	if err != nil {
		return nil, err
	}
	out := make([]RoomSummary, 0, len(resp.Rooms))
	for _, r := range resp.Rooms {
		out = append(out, RoomSummary{
			ID:          r.Id,
			Title:       r.Title,
			Description: r.Description,
			DangerLevel: r.DangerLevel,
		})
	}
	return out, nil
}

// UpdateRoom calls AdminUpdateRoom with the non-empty fields from patch.
func (g *grpcWorldEditor) UpdateRoom(roomID string, patch RoomPatch) error {
	_, err := g.client.AdminUpdateRoom(context.Background(), &gamev1.AdminUpdateRoomRequest{
		RoomId:      roomID,
		Title:       patch.Title,
		Description: patch.Description,
		DangerLevel: patch.DangerLevel,
	})
	return err
}

// AllNPCTemplates calls AdminListNPCTemplates and maps the result to []NPCTemplate.
func (g *grpcWorldEditor) AllNPCTemplates() []NPCTemplate {
	resp, err := g.client.AdminListNPCTemplates(context.Background(), &gamev1.AdminListNPCTemplatesRequest{})
	if err != nil {
		g.logger.Error("AdminListNPCTemplates failed", zap.Error(err))
		return []NPCTemplate{}
	}
	out := make([]NPCTemplate, 0, len(resp.Templates))
	for _, t := range resp.Templates {
		out = append(out, NPCTemplate{
			ID:    t.Id,
			Name:  t.Name,
			Level: int(t.Level),
			Type:  t.Type,
		})
	}
	return out
}

// SpawnNPC calls AdminSpawnNPC with the given template ID, room ID, and count.
func (g *grpcWorldEditor) SpawnNPC(templateID, roomID string, count int) (int, error) {
	resp, err := g.client.AdminSpawnNPC(context.Background(), &gamev1.AdminSpawnNPCRequest{
		TemplateId: templateID,
		RoomId:     roomID,
		Count:      int32(count),
	})
	if err != nil {
		return 0, fmt.Errorf("AdminSpawnNPC: %w", err)
	}
	return int(resp.SpawnedCount), nil
}
