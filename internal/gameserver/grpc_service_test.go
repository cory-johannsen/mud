package gameserver

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// stubSkillsRepo is an in-memory implementation of CharacterSkillsGetter for testing.
type stubSkillsRepo struct {
	data map[int64]map[string]string
}

func (r *stubSkillsRepo) GetAll(_ context.Context, characterID int64) (map[string]string, error) {
	if m, ok := r.data[characterID]; ok {
		return m, nil
	}
	return map[string]string{}, nil
}

func (r *stubSkillsRepo) HasSkills(_ context.Context, characterID int64) (bool, error) {
	m, ok := r.data[characterID]
	return ok && len(m) > 0, nil
}

func (r *stubSkillsRepo) SetAll(_ context.Context, characterID int64, skills map[string]string) error {
	r.data[characterID] = skills
	return nil
}

// stubFeatsRepo is an in-memory implementation of CharacterFeatsGetter for testing.
type stubFeatsRepo struct {
	data map[int64][]string
}

func (r *stubFeatsRepo) GetAll(_ context.Context, characterID int64) ([]string, error) {
	return r.data[characterID], nil
}

// stubClassFeaturesRepo is an in-memory implementation of CharacterClassFeaturesGetter for testing.
type stubClassFeaturesRepo struct {
	data map[int64][]string
}

func (r *stubClassFeaturesRepo) GetAll(_ context.Context, characterID int64) ([]string, error) {
	return r.data[characterID], nil
}

// TestStartZoneTicks_RespawnIntegration verifies that StartZoneTicks drives
// RespawnManager.Tick, which spawns a pending NPC into the target room.
func TestStartZoneTicks_RespawnIntegration(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)

	// Build a template and npc manager.
	tmpl := &npc.Template{
		ID:    "rat",
		Name:  "Rat",
		MaxHP: 5,
		AC:    10,
	}
	npcMgr := npc.NewManager()

	// Configure respawn: room_a, cap=1, delay=10ms.
	roomSpawns := map[string][]npc.RoomSpawn{
		"room_a": {
			{TemplateID: "rat", Max: 1, RespawnDelay: 10 * time.Millisecond},
		},
	}
	templates := map[string]*npc.Template{"rat": tmpl}
	respawnMgr := npc.NewRespawnManager(roomSpawns, templates)

	// Schedule a respawn that will be ready after 10ms.
	respawnMgr.Schedule("rat", "room_a", time.Now(), 10*time.Millisecond)

	logger := zaptest.NewLogger(t)
	cmdRegistry := command.DefaultRegistry()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil)
	chatHandler := NewChatHandler(sessMgr)
	npcHandler := NewNPCHandler(npcMgr, sessMgr)

	svc := NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		nil, nil, npcHandler, npcMgr, nil, nil,
		respawnMgr, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	zm := NewZoneTickManager(50 * time.Millisecond)
	svc.StartZoneTicks(ctx, zm, nil)

	// Wait long enough for at least one tick to fire and drain the respawn queue.
	time.Sleep(200 * time.Millisecond)

	instances := npcMgr.InstancesInRoom("room_a")
	require.Len(t, instances, 1, "expected 1 NPC instance in room_a after respawn tick")
	assert.Equal(t, "rat", instances[0].TemplateID)

	cancel()
}

// testGRPCServer starts an in-process gRPC server and returns a connected client.
func testGRPCServer(t *testing.T) (gamev1.GameServiceClient, *session.Manager) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	svc := NewGameServiceServer(worldMgr, sessMgr, cmdRegistry, worldHandler, chatHandler, logger, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", nil, nil, nil, nil, nil, nil, nil, nil)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	gamev1.RegisterGameServiceServer(grpcServer, svc)

	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() { grpcServer.Stop() })

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	return gamev1.NewGameServiceClient(conn), sessMgr
}

func joinWorld(t *testing.T, stream gamev1.GameService_SessionClient, uid, username string) *gamev1.RoomView {
	t.Helper()
	err := stream.Send(&gamev1.ClientMessage{
		RequestId: "join",
		Payload: &gamev1.ClientMessage_JoinWorld{
			JoinWorld: &gamev1.JoinWorldRequest{
				Uid:      uid,
				Username: username,
			},
		},
	})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)
	roomView := resp.GetRoomView()
	require.NotNil(t, roomView, "expected RoomView after join")
	return roomView
}

func TestGRPCService_JoinWorld(t *testing.T) {
	client, _ := testGRPCServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	view := joinWorld(t, stream, "u1", "Alice")
	assert.Equal(t, "room_a", view.RoomId)
	assert.Equal(t, "Room A", view.Title)
}

func TestGRPCService_Move(t *testing.T) {
	client, _ := testGRPCServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorld(t, stream, "u1", "Alice")

	// Move north
	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "move1",
		Payload: &gamev1.ClientMessage_Move{
			Move: &gamev1.MoveRequest{Direction: "north"},
		},
	})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)
	view := resp.GetRoomView()
	require.NotNil(t, view)
	assert.Equal(t, "room_b", view.RoomId)
}

func TestGRPCService_Look(t *testing.T) {
	client, _ := testGRPCServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorld(t, stream, "u1", "Alice")

	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "look1",
		Payload:   &gamev1.ClientMessage_Look{Look: &gamev1.LookRequest{}},
	})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)
	view := resp.GetRoomView()
	require.NotNil(t, view)
	assert.Equal(t, "room_a", view.RoomId)
}

func TestGRPCService_Exits(t *testing.T) {
	client, _ := testGRPCServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorld(t, stream, "u1", "Alice")

	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "exits1",
		Payload:   &gamev1.ClientMessage_Exits{Exits: &gamev1.ExitsRequest{}},
	})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)
	exitList := resp.GetExitList()
	require.NotNil(t, exitList)
	require.Len(t, exitList.Exits, 1)
	assert.Equal(t, "north", exitList.Exits[0].Direction)
}

func TestGRPCService_Say(t *testing.T) {
	client, _ := testGRPCServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorld(t, stream, "u1", "Alice")

	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "say1",
		Payload: &gamev1.ClientMessage_Say{
			Say: &gamev1.SayRequest{Message: "hello"},
		},
	})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)
	msg := resp.GetMessage()
	require.NotNil(t, msg)
	assert.Equal(t, "Alice", msg.Sender)
	assert.Equal(t, "hello", msg.Content)
	assert.Equal(t, gamev1.MessageType_MESSAGE_TYPE_SAY, msg.Type)
}

func TestGRPCService_Who(t *testing.T) {
	client, _ := testGRPCServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorld(t, stream, "u1", "Alice")

	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "who1",
		Payload:   &gamev1.ClientMessage_Who{Who: &gamev1.WhoRequest{}},
	})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)
	playerList := resp.GetPlayerList()
	require.NotNil(t, playerList)
	require.NotEmpty(t, playerList.Players)
	names := make([]string, 0, len(playerList.Players))
	for _, p := range playerList.Players {
		names = append(names, p.Name)
	}
	assert.Contains(t, names, "Alice")
}

func TestGRPCService_Emote(t *testing.T) {
	client, _ := testGRPCServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorld(t, stream, "u1", "Alice")

	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "emote1",
		Payload: &gamev1.ClientMessage_Emote{
			Emote: &gamev1.EmoteRequest{Action: "waves"},
		},
	})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)
	msg := resp.GetMessage()
	require.NotNil(t, msg)
	assert.Equal(t, "Alice", msg.Sender)
	assert.Equal(t, "waves", msg.Content)
	assert.Equal(t, gamev1.MessageType_MESSAGE_TYPE_EMOTE, msg.Type)
}

func TestGRPCService_Quit(t *testing.T) {
	client, _ := testGRPCServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorld(t, stream, "u1", "Alice")

	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "quit1",
		Payload:   &gamev1.ClientMessage_Quit{Quit: &gamev1.QuitRequest{}},
	})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)
	disc := resp.GetDisconnected()
	require.NotNil(t, disc)
}

func TestGRPCService_MoveError(t *testing.T) {
	client, _ := testGRPCServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorld(t, stream, "u1", "Alice")

	// Try to move in a direction with no exit
	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "move_fail",
		Payload: &gamev1.ClientMessage_Move{
			Move: &gamev1.MoveRequest{Direction: "west"},
		},
	})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)
	errEvt := resp.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "no exit")
}

func TestGRPCService_Broadcast(t *testing.T) {
	client, sessMgr := testGRPCServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Player 1
	stream1, err := client.Session(ctx)
	require.NoError(t, err)
	joinWorld(t, stream1, "u1", "Alice")

	// Player 2
	stream2, err := client.Session(ctx)
	require.NoError(t, err)

	// Player 2 joins — this should broadcast arrival to player 1
	joinWorld(t, stream2, "u2", "Bob")

	// Player 1 should get a room event via the entity channel → forwarded by forwardEvents
	// Give it a moment for the broadcast to propagate
	time.Sleep(50 * time.Millisecond)

	// Verify both players are tracked
	assert.Equal(t, 2, sessMgr.PlayerCount())
	players := sessMgr.PlayersInRoom("room_a")
	assert.Len(t, players, 2)
}

// testGRPCServerWithCharData starts an in-process gRPC server wired with stub repos
// and in-memory ruleset data for skills, feats, and class features.
func testGRPCServerWithCharData(t *testing.T) (gamev1.GameServiceClient, *session.Manager) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	// Skills.
	allSkills := []*ruleset.Skill{
		{ID: "acrobatics", Name: "Acrobatics", Ability: "dex"},
		{ID: "athletics", Name: "Athletics", Ability: "str"},
	}
	skillsRepo := &stubSkillsRepo{
		data: map[int64]map[string]string{
			// characterID 0 (the default in test sessions) gets one trained skill.
			0: {"acrobatics": "trained"},
		},
	}

	// Feats.
	allFeats := []*ruleset.Feat{
		{ID: "quick-draw", Name: "Quick Draw", Active: true, Description: "Draw fast.", ActivateText: "Activate."},
	}
	featRegistry := ruleset.NewFeatRegistry(allFeats)
	featsRepo := &stubFeatsRepo{
		data: map[int64][]string{
			0: {"quick-draw"},
		},
	}

	// Class features.
	allClassFeatures := []*ruleset.ClassFeature{
		{ID: "battle-cry", Name: "Battle Cry", Archetype: "warrior", Job: "soldier", Active: true, Description: "Cry out.", ActivateText: "Use it."},
	}
	cfRegistry := ruleset.NewClassFeatureRegistry(allClassFeatures)
	cfRepo := &stubClassFeaturesRepo{
		data: map[int64][]string{
			0: {"battle-cry"},
		},
	}

	svc := NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		allSkills, skillsRepo,
		allFeats, featRegistry, featsRepo,
		allClassFeatures, cfRegistry, cfRepo,
	)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	gamev1.RegisterGameServiceServer(grpcServer, svc)
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() { grpcServer.Stop() })

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	return gamev1.NewGameServiceClient(conn), sessMgr
}

// TestHandleChar verifies that handleChar populates Skills, Feats, and ClassFeatures
// on the returned CharacterSheetView.
func TestHandleChar(t *testing.T) {
	client, _ := testGRPCServerWithCharData(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorld(t, stream, "u1", "Alice")

	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "char1",
		Payload:   &gamev1.ClientMessage_CharSheet{CharSheet: &gamev1.CharacterSheetRequest{}},
	})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)

	sheet := resp.GetCharacterSheet()
	require.NotNil(t, sheet, "expected CharacterSheetView response")

	// Skills: two skills expected (acrobatics trained, athletics untrained).
	require.Len(t, sheet.Skills, 2, "expected 2 skill entries")
	skillByID := make(map[string]*gamev1.SkillEntry, len(sheet.Skills))
	for _, sk := range sheet.Skills {
		skillByID[sk.SkillId] = sk
	}
	require.Contains(t, skillByID, "acrobatics")
	assert.Equal(t, "trained", skillByID["acrobatics"].Proficiency)
	assert.Equal(t, "dex", skillByID["acrobatics"].Ability)
	require.Contains(t, skillByID, "athletics")
	assert.Equal(t, "untrained", skillByID["athletics"].Proficiency)

	// Feats: one feat expected.
	require.Len(t, sheet.Feats, 1, "expected 1 feat entry")
	assert.Equal(t, "quick-draw", sheet.Feats[0].FeatId)
	assert.Equal(t, "Quick Draw", sheet.Feats[0].Name)
	assert.True(t, sheet.Feats[0].Active)

	// Class features: one class feature expected.
	require.Len(t, sheet.ClassFeatures, 1, "expected 1 class feature entry")
	assert.Equal(t, "battle-cry", sheet.ClassFeatures[0].FeatureId)
	assert.Equal(t, "Battle Cry", sheet.ClassFeatures[0].Name)
	assert.Equal(t, "warrior", sheet.ClassFeatures[0].Archetype)
	assert.Equal(t, "soldier", sheet.ClassFeatures[0].Job)
}
