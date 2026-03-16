package gameserver

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/skillcheck"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/scripting"
)

// fixedDiceSource always returns a fixed value for Intn, for deterministic tests.
type fixedDiceSource struct{ val int }

func (f *fixedDiceSource) Intn(_ int) int { return f.val }

// stubSkillsRepo is an in-memory implementation of CharacterSkillsRepository for testing.
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

func (r *stubSkillsRepo) UpgradeSkill(_ context.Context, characterID int64, skillID, newRank string) error {
	if r.data == nil {
		r.data = make(map[int64]map[string]string)
	}
	if r.data[characterID] == nil {
		r.data[characterID] = make(map[string]string)
	}
	r.data[characterID][skillID] = newRank
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
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	npcHandler := NewNPCHandler(npcMgr, sessMgr)

	svc := NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		nil, nil, npcHandler, npcMgr, nil, nil,
		respawnMgr, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
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
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	svc := NewGameServiceServer(worldMgr, sessMgr, cmdRegistry, worldHandler, chatHandler, logger, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	// Skills.
	allSkills := []*ruleset.Skill{
		{ID: "acrobatics", Name: "Acrobatics", Ability: "dex"},
		{ID: "muscle", Name: "Muscle", Ability: "brutality"},
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
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		allSkills, skillsRepo, nil,
		allFeats, featRegistry, featsRepo,
		allClassFeatures, cfRegistry, cfRepo, nil, nil, nil, nil,
		nil, nil,
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
	require.Contains(t, skillByID, "muscle")
	assert.Equal(t, "untrained", skillByID["muscle"].Proficiency)

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

// TestApplyRoomSkillChecks_OnEnter_Success verifies that applyRoomSkillChecks returns the
// success message when the player rolls high enough.
//
// Precondition: room has a parkour on_enter skill check DC 10; player has parkour=trained,
// Quickness=14 (mod=+2); dice source returns 9 (so roll=10, total=10+2+2=14 >= DC10 → success).
func TestApplyRoomSkillChecks_OnEnter_Success(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	// Build a room with an on_enter skill check for parkour.
	room := &world.Room{
		ID:          "room_skill",
		ZoneID:      "test",
		Title:       "Skill Room",
		Description: "A room requiring parkour.",
		Exits:       []world.Exit{},
		Properties:  map[string]string{},
		SkillChecks: []skillcheck.TriggerDef{
			{
				Skill:   "parkour",
				DC:      10,
				Trigger: "on_enter",
				Outcomes: skillcheck.OutcomeMap{
					Success: &skillcheck.Outcome{Message: "You vault it."},
					Failure: &skillcheck.Outcome{Message: "You stumble."},
				},
			},
		},
	}

	// Fixed dice: Intn(20) always returns 9, so d20 roll = 10.
	// Quickness=14 → abilityModFrom(14) = (14-10)/2 = 2.
	// parkour trained → profBonus=2.
	// total = 10 + 2 + 2 = 14 >= DC 10 → success.
	src := &fixedDiceSource{val: 9}
	roller := dice.NewLoggedRoller(src, logger)

	// Register the parkour skill so abilityScoreForSkill can find it.
	skills := []*ruleset.Skill{
		{ID: "parkour", Name: "Parkour", Ability: "quickness"},
	}

	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	// Add a player session with Skills and Abilities set.
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_skill",
		Username:  "Tester",
		CharName:  "Tester",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{Quickness: 14},
		Role:      "player",
	})
	require.NoError(t, err)
	sess.Skills = map[string]string{"parkour": "trained"}

	msgs := svc.applyRoomSkillChecks("u_skill", room)
	require.Len(t, msgs, 2, "expected detail line and outcome message")
	assert.Equal(t, "Parkour check (DC 10): rolled 10+2=14 — success.", msgs[0])
	assert.Equal(t, "You vault it.", msgs[1])
}

// TestApplyRoomSkillChecks_OnEnter_Failure verifies that applyRoomSkillChecks returns the
// failure message when the player rolls too low.
//
// Precondition: same room/player setup; dice source returns 0 (roll=1, total=1+2+2=5 < DC10 → failure).
func TestApplyRoomSkillChecks_OnEnter_Failure(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	room := &world.Room{
		ID:          "room_skill",
		ZoneID:      "test",
		Title:       "Skill Room",
		Description: "A room requiring parkour.",
		Exits:       []world.Exit{},
		Properties:  map[string]string{},
		SkillChecks: []skillcheck.TriggerDef{
			{
				Skill:   "parkour",
				DC:      10,
				Trigger: "on_enter",
				Outcomes: skillcheck.OutcomeMap{
					Success: &skillcheck.Outcome{Message: "You vault it."},
					Failure: &skillcheck.Outcome{Message: "You stumble."},
				},
			},
		},
	}

	// Fixed dice: Intn(20) returns 0, so d20 roll = 1.
	// total = 1 + 2 + 2 = 5 < DC 10 → failure.
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	skills := []*ruleset.Skill{
		{ID: "parkour", Name: "Parkour", Ability: "quickness"},
	}

	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_skill2",
		Username:  "Tester2",
		CharName:  "Tester2",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{Quickness: 14},
		Role:      "player",
	})
	require.NoError(t, err)
	sess.Skills = map[string]string{"parkour": "trained"}

	msgs := svc.applyRoomSkillChecks("u_skill2", room)
	require.Len(t, msgs, 2, "expected detail line and outcome message")
	assert.Equal(t, "Parkour check (DC 10): rolled 1+2=5 — failure.", msgs[0])
	assert.Equal(t, "You stumble.", msgs[1])
}

// TestApplyRoomSkillChecks_NoOnEnterTriggers verifies that non-on_enter triggers are ignored.
func TestApplyRoomSkillChecks_NoOnEnterTriggers(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	room := &world.Room{
		ID:          "room_no_trigger",
		ZoneID:      "test",
		Title:       "No Trigger Room",
		Description: "No on_enter checks here.",
		Exits:       []world.Exit{},
		Properties:  map[string]string{},
		SkillChecks: []skillcheck.TriggerDef{
			{
				Skill:   "parkour",
				DC:      10,
				Trigger: "on_use", // different trigger type — must be ignored
				Outcomes: skillcheck.OutcomeMap{
					Success: &skillcheck.Outcome{Message: "Used."},
				},
			},
		},
	}

	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_skill3",
		Username:  "Tester3",
		CharName:  "Tester3",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	msgs := svc.applyRoomSkillChecks("u_skill3", room)
	assert.Empty(t, msgs, "on_use trigger must not fire on room enter")
}

// TestAbilityModFrom verifies the ability modifier calculation.
func TestAbilityModFrom(t *testing.T) {
	cases := []struct {
		score    int
		expected int
	}{
		{10, 0},
		{11, 0},
		{12, 1},
		{14, 2},
		{18, 4},
		{9, -1},
		{8, -1},
		{7, -2},
		{6, -2},
		{1, -5},
	}
	for _, tc := range cases {
		got := abilityModFrom(tc.score)
		assert.Equal(t, tc.expected, got, "abilityModFrom(%d)", tc.score)
	}
}

// writeTempLuaFile writes a single Lua source file into a temp directory and returns the
// directory path.
//
// Precondition: t must be non-nil; src must be valid Lua source.
// Postcondition: Returns the directory containing the written file.
func writeTempLuaFile(t *testing.T, filename, src string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(src), 0644))
	return dir
}

// TestApplyRoomSkillChecks_LuaHookSignature verifies that the on_skill_check Lua hook is
// called with exactly five arguments: uid (string), skill_id (string), total (number),
// dc (number), and outcome (string).
//
// Precondition: room has a parkour on_enter check DC 10; player has quickness=14, parkour=trained;
// dice source returns 9 (total=14, success).
// Postcondition: the recorded Lua arguments match (uid, skill_id, total, dc, outcome_string).
func TestApplyRoomSkillChecks_LuaHookSignature(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	// Lua script records the five on_skill_check arguments into globals for inspection.
	luaSrc := `
captured_uid     = ""
captured_skill   = ""
captured_total   = 0
captured_dc      = 0
captured_outcome = ""

function on_skill_check(uid, skill_id, total, dc, outcome)
    captured_uid     = uid
    captured_skill   = skill_id
    captured_total   = total
    captured_dc      = dc
    captured_outcome = outcome
end
`
	src := &fixedDiceSource{val: 9} // d20=10; quickness mod=+2; trained=+2 => total=14
	roller := dice.NewLoggedRoller(src, logger)

	scriptMgr := scripting.NewManager(roller, logger)
	dir := writeTempLuaFile(t, "hooks.lua", luaSrc)
	require.NoError(t, scriptMgr.LoadZone("test", dir, 0))

	skills := []*ruleset.Skill{
		{ID: "parkour", Name: "Parkour", Ability: "quickness"},
	}

	room := &world.Room{
		ID:          "room_lua_hook",
		ZoneID:      "test",
		Title:       "Hook Room",
		Description: "Tests Lua hook signature.",
		Exits:       []world.Exit{},
		Properties:  map[string]string{},
		SkillChecks: []skillcheck.TriggerDef{
			{
				Skill:   "parkour",
				DC:      10,
				Trigger: "on_enter",
				Outcomes: skillcheck.OutcomeMap{
					Success: &skillcheck.Outcome{Message: "You vault it."},
					Failure: &skillcheck.Outcome{Message: "You stumble."},
				},
			},
		},
	}

	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, nil, nil, scriptMgr,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_lua_hook",
		Username:  "Hooker",
		CharName:  "Hooker",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{Quickness: 14},
		Role:      "player",
	})
	require.NoError(t, err)
	sess.Skills = map[string]string{"parkour": "trained"}

	msgs := svc.applyRoomSkillChecks("u_lua_hook", room)
	require.Len(t, msgs, 2, "expected detail line and outcome message")
	assert.Equal(t, "Parkour check (DC 10): rolled 10+2=14 — success.", msgs[0])
	assert.Equal(t, "You vault it.", msgs[1])

	// Inspect captured globals from Lua to verify the hook was called with the correct signature.
	capturedUID, _ := scriptMgr.CallHook("test", "tostring", lua.LString("u_lua_hook"))
	_ = capturedUID // used below via direct global reads

	// Read each captured global by calling a small Lua accessor.
	checkGlobal := func(varName string) lua.LValue {
		ret, err2 := scriptMgr.CallHook("test", "tostring",
			lua.LString(varName)) // tostring is a placeholder; use DoString pattern instead
		_ = ret
		_ = err2
		return lua.LNil
	}
	_ = checkGlobal

	// Use a verifier hook to read the captured values back to Go.
	verifyLua := `
function get_captured()
    return captured_uid, captured_skill, captured_total, captured_dc, captured_outcome
end
`
	verifyDir := writeTempLuaFile(t, "verify.lua", verifyLua)
	require.NoError(t, scriptMgr.LoadZone("test", verifyDir, 0))

	// After LoadZone with a new dir the previous state is replaced — use a combined script instead.
	// Restart: embed the verifier in the original script.
	combinedSrc := luaSrc + `
function get_captured_uid()     return captured_uid     end
function get_captured_skill()   return captured_skill   end
function get_captured_total()   return captured_total   end
function get_captured_dc()      return captured_dc      end
function get_captured_outcome() return captured_outcome end
`
	scriptMgr2 := scripting.NewManager(roller, logger)
	dir2 := writeTempLuaFile(t, "hooks.lua", combinedSrc)
	require.NoError(t, scriptMgr2.LoadZone("test", dir2, 0))

	svc2 := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, nil, nil, scriptMgr2,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	sess2, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_lua_hook2",
		Username:  "Hooker2",
		CharName:  "Hooker2",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{Quickness: 14},
		Role:      "player",
	})
	require.NoError(t, err)
	sess2.Skills = map[string]string{"parkour": "trained"}

	room2 := *room
	room2.ID = "room_lua_hook2"
	msgs2 := svc2.applyRoomSkillChecks("u_lua_hook2", &room2)
	require.Len(t, msgs2, 2, "expected detail line and outcome message")
	assert.Equal(t, "Parkour check (DC 10): rolled 10+2=14 — success.", msgs2[0])
	assert.Equal(t, "You vault it.", msgs2[1])

	gotUID, _ := scriptMgr2.CallHook("test", "get_captured_uid")
	gotSkill, _ := scriptMgr2.CallHook("test", "get_captured_skill")
	gotTotal, _ := scriptMgr2.CallHook("test", "get_captured_total")
	gotDC, _ := scriptMgr2.CallHook("test", "get_captured_dc")
	gotOutcome, _ := scriptMgr2.CallHook("test", "get_captured_outcome")

	assert.Equal(t, lua.LString("u_lua_hook2"), gotUID, "on_skill_check arg[1]: uid")
	assert.Equal(t, lua.LString("parkour"), gotSkill, "on_skill_check arg[2]: skill_id")
	assert.Equal(t, lua.LNumber(14), gotTotal, "on_skill_check arg[3]: total")
	assert.Equal(t, lua.LNumber(10), gotDC, "on_skill_check arg[4]: dc")
	assert.Equal(t, lua.LString("success"), gotOutcome, "on_skill_check arg[5]: outcome")
}

// TestApplyRoomSkillChecks_DamageEffect verifies that when a skill check outcome carries a
// damage effect the player's CurrentHP is reduced by the rolled damage.
//
// Precondition: room has a parkour on_enter check DC 10 with failure outcome Effect{Type:"damage",Formula:"1d4"};
// player has quickness=10, no proficiency (untrained); dice source returns 0 (roll=1, total=1 < DC10 → failure);
// the damage roll also uses the fixed dice source (Intn(4) returns 0 → 1 damage).
// Postcondition: sess.CurrentHP == 9 (10 - 1).
func TestApplyRoomSkillChecks_DamageEffect(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	// Fixed dice: Intn(n) always returns 0 regardless of n.
	// d20 roll = 1, total = 1 + 0 (abilityMod for score 10) + 0 (untrained) = 1 < DC10 → failure.
	// Damage formula "1d4": Intn(4) = 0 → damage = 1.
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	skills := []*ruleset.Skill{
		{ID: "parkour", Name: "Parkour", Ability: "quickness"},
	}

	room := &world.Room{
		ID:          "room_dmg_effect",
		ZoneID:      "test",
		Title:       "Damage Effect Room",
		Description: "Failure here hurts.",
		Exits:       []world.Exit{},
		Properties:  map[string]string{},
		SkillChecks: []skillcheck.TriggerDef{
			{
				Skill:   "parkour",
				DC:      10,
				Trigger: "on_enter",
				Outcomes: skillcheck.OutcomeMap{
					Success: &skillcheck.Outcome{Message: "You vault it."},
					Failure: &skillcheck.Outcome{
						Message: "You stumble and take damage.",
						Effect:  &skillcheck.Effect{Type: "damage", Formula: "1d4"},
					},
				},
			},
		},
	}

	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_dmg_effect",
		Username:  "Bruiser",
		CharName:  "Bruiser",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{Quickness: 10},
		Role:      "player",
	})
	require.NoError(t, err)
	// No skills set → untrained (profBonus = 0).

	msgs := svc.applyRoomSkillChecks("u_dmg_effect", room)
	require.Len(t, msgs, 2, "expected detail line and outcome message")
	assert.Equal(t, "Parkour check (DC 10): rolled 1+0=1 — failure.", msgs[0])
	assert.Equal(t, "You stumble and take damage.", msgs[1])

	// Intn(4)=0 → dice result is 1; damage total = 1.
	assert.Equal(t, 9, sess.CurrentHP, "CurrentHP must be reduced by the damage roll")
}

// TestApplyNPCSkillChecks_OnGreet_Success verifies that applyNPCSkillChecks fires
// on_greet triggers for NPCs in the room and returns the success message when the
// player's roll exceeds the DC.
//
// Setup: NPC template has smooth_talk on_greet DC 12. Player has Flair=16 (mod=+3)
// and smooth_talk "trained" (profBonus=2). dice source returns 9 → roll=10, total=10+3+2=15 >= 12 → success.
func TestApplyNPCSkillChecks_OnGreet_Success(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	npcMgr := npc.NewManager()

	tmpl := &npc.Template{
		ID:          "charming_vendor",
		Name:        "Charming Vendor",
		Description: "A vendor with a warm smile.",
		Level:       1,
		MaxHP:       10,
		AC:          10,
		SkillChecks: []skillcheck.TriggerDef{
			{
				Skill:   "smooth_talk",
				DC:      12,
				Trigger: "on_greet",
				Outcomes: skillcheck.OutcomeMap{
					Success: &skillcheck.Outcome{Message: "They warm up to you."},
					Failure: &skillcheck.Outcome{Message: "They sneer."},
				},
			},
		},
	}
	_, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	// Fixed dice: Intn(20) always returns 9, so d20 roll = 10.
	// Flair=16 → abilityModFrom(16) = (16-10)/2 = 3.
	// smooth_talk trained → profBonus=2.
	// total = 10 + 3 + 2 = 15 >= DC 12 → success.
	src := &fixedDiceSource{val: 9}
	roller := dice.NewLoggedRoller(src, logger)

	skills := []*ruleset.Skill{
		{ID: "smooth_talk", Name: "Smooth Talk", Ability: "flair"},
	}

	npcHandler := NewNPCHandler(npcMgr, sessMgr)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, npcHandler, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_npc_greet",
		Username:  "Diplomat",
		CharName:  "Diplomat",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{Flair: 16},
		Role:      "player",
	})
	require.NoError(t, err)
	sess.Skills = map[string]string{"smooth_talk": "trained"}

	msgs := svc.applyNPCSkillChecks("u_npc_greet", "room_a")
	require.Len(t, msgs, 2, "expected detail line and outcome message from NPC on_greet check")
	assert.Equal(t, "Smooth Talk check (DC 12): rolled 10+3=15 — success.", msgs[0])
	assert.Equal(t, "Charming Vendor: They warm up to you.", msgs[1])
}

// TestApplyNPCSkillChecks_OnGreet_Failure verifies that applyNPCSkillChecks returns
// the failure message when the player's roll falls short of the DC.
//
// Setup: NPC template has smooth_talk on_greet DC 16. dice source returns 4 → roll=5.
// Flair=16 → mod=+3; trained → profBonus=2. total=5+3+2=10 < 16 and 10 > 16-10=6 → regular Failure.
func TestApplyNPCSkillChecks_OnGreet_Failure(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	npcMgr := npc.NewManager()

	tmpl := &npc.Template{
		ID:          "stern_guard",
		Name:        "Stern Guard",
		Description: "A guard who doesn't like visitors.",
		Level:       1,
		MaxHP:       10,
		AC:          10,
		SkillChecks: []skillcheck.TriggerDef{
			{
				Skill:   "smooth_talk",
				DC:      16,
				Trigger: "on_greet",
				Outcomes: skillcheck.OutcomeMap{
					Success: &skillcheck.Outcome{Message: "They nod reluctantly."},
					Failure: &skillcheck.Outcome{Message: "They sneer."},
				},
			},
		},
	}
	_, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	// Fixed dice: Intn(20) always returns 4, so d20 roll = 5.
	// Flair=16 → mod=+3; trained → profBonus=2. total=5+3+2=10.
	// CritFailure threshold = DC-10 = 6. total=10 > 6, so it is a regular Failure (not CritFailure).
	// 10 < 16 → Failure.
	src := &fixedDiceSource{val: 4}
	roller := dice.NewLoggedRoller(src, logger)

	skills := []*ruleset.Skill{
		{ID: "smooth_talk", Name: "Smooth Talk", Ability: "flair"},
	}

	npcHandler := NewNPCHandler(npcMgr, sessMgr)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, npcHandler, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_npc_greet_fail",
		Username:  "Diplomat2",
		CharName:  "Diplomat2",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{Flair: 16},
		Role:      "player",
	})
	require.NoError(t, err)
	sess.Skills = map[string]string{"smooth_talk": "trained"}

	msgs := svc.applyNPCSkillChecks("u_npc_greet_fail", "room_a")
	require.Len(t, msgs, 2, "expected detail line and outcome message from NPC on_greet check")
	assert.Equal(t, "Smooth Talk check (DC 16): rolled 5+3=10 — failure.", msgs[0])
	assert.Equal(t, "Stern Guard: They sneer.", msgs[1])
}

// TestApplyNPCSkillChecks_NoNPCs verifies that applyNPCSkillChecks returns nil when
// there are no NPCs in the room.
func TestApplyNPCSkillChecks_NoNPCs(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	npcMgr := npc.NewManager()
	npcHandler := NewNPCHandler(npcMgr, sessMgr)

	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, npcHandler, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_no_npc",
		Username:  "Wanderer",
		CharName:  "Wanderer",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Role:      "player",
	})
	require.NoError(t, err)

	msgs := svc.applyNPCSkillChecks("u_no_npc", "room_a")
	assert.Nil(t, msgs, "expected nil when no NPCs present")
}

// TestApplyNPCSkillChecks_NonGreetTriggerIgnored verifies that skill checks with
// triggers other than on_greet are not fired by applyNPCSkillChecks.
func TestApplyNPCSkillChecks_NonGreetTriggerIgnored(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	npcMgr := npc.NewManager()

	tmpl := &npc.Template{
		ID:          "passive_npc",
		Name:        "Passive NPC",
		Description: "Does nothing on enter.",
		Level:       1,
		MaxHP:       10,
		AC:          10,
		SkillChecks: []skillcheck.TriggerDef{
			{
				Skill:   "smooth_talk",
				DC:      10,
				Trigger: "on_enter", // NOT on_greet — should be ignored
				Outcomes: skillcheck.OutcomeMap{
					Success: &skillcheck.Outcome{Message: "Wrong trigger fired."},
				},
			},
		},
	}
	_, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	npcHandler := NewNPCHandler(npcMgr, sessMgr)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, npcHandler, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_non_greet",
		Username:  "Explorer",
		CharName:  "Explorer",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Role:      "player",
	})
	require.NoError(t, err)

	msgs := svc.applyNPCSkillChecks("u_non_greet", "room_a")
	assert.Empty(t, msgs, "expected no messages when NPC has no on_greet triggers")
}

// TestApplyNPCSkillChecks_DamageEffect verifies that when an NPC on_greet skill check
// outcome carries a damage effect the player's CurrentHP is reduced by the rolled damage.
//
// Precondition: NPC template has smooth_talk on_greet DC 10 with failure outcome
// Effect{Type:"damage", Formula:"1d4"}; player has Flair=10 (mod=0), untrained (profBonus=0);
// fixed dice source returns 0 → d20 roll=1, total=1; CritFailure threshold=DC-10=0, so
// total=1 > 0 → regular Failure (not CritFailure); total=1 < DC10 → failure branch taken.
// Damage formula "1d4": Intn(4)=0 → damage=1.
// Postcondition: sess.CurrentHP == 9 (10 - 1).
func TestApplyNPCSkillChecks_DamageEffect(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	npcMgr := npc.NewManager()

	tmpl := &npc.Template{
		ID:          "brutal_guard",
		Name:        "Brutal Guard",
		Description: "A guard who attacks on a failed greeting.",
		Level:       1,
		MaxHP:       10,
		AC:          10,
		SkillChecks: []skillcheck.TriggerDef{
			{
				Skill:   "smooth_talk",
				DC:      10,
				Trigger: "on_greet",
				Outcomes: skillcheck.OutcomeMap{
					Success: &skillcheck.Outcome{Message: "They let you pass."},
					Failure: &skillcheck.Outcome{
						Message: "They strike you!",
						Effect:  &skillcheck.Effect{Type: "damage", Formula: "1d4"},
					},
				},
			},
		},
	}
	_, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	// Fixed dice: Intn(n) always returns 0.
	// d20 roll=1; Flair=10 → mod=0; untrained → profBonus=0; total=1.
	// CritFailure threshold=DC-10=0; total=1 > 0 → regular Failure.
	// total=1 < DC10 → Failure branch taken.
	// Damage formula "1d4": Intn(4)=0 → damage=1.
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	skills := []*ruleset.Skill{
		{ID: "smooth_talk", Name: "Smooth Talk", Ability: "flair"},
	}

	npcHandler := NewNPCHandler(npcMgr, sessMgr)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, npcHandler, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_npc_dmg_effect",
		Username:  "Victim",
		CharName:  "Victim",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{Flair: 10},
		Role:      "player",
	})
	require.NoError(t, err)
	// No skills set → untrained (profBonus = 0).

	msgs := svc.applyNPCSkillChecks("u_npc_dmg_effect", "room_a")
	require.Len(t, msgs, 2, "expected detail line and outcome message")
	assert.Equal(t, "Smooth Talk check (DC 10): rolled 1+0=1 — failure.", msgs[0])
	assert.Equal(t, "Brutal Guard: They strike you!", msgs[1])

	// Intn(4)=0 → dice result=1; damage total=1; 10-1=9.
	assert.Equal(t, 9, sess.CurrentHP, "CurrentHP must be reduced by the NPC on_greet damage effect")
}

// TestProperty_AbilityModFrom_MatchesFloor verifies that abilityModFrom matches
// the floor((score-10)/2) reference formula for all scores in [-10, 30].
func TestProperty_AbilityModFrom_MatchesFloor(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		score := rapid.IntRange(-10, 30).Draw(rt, "score")
		got := abilityModFrom(score)
		// Reference: floor((score-10) / 2)
		n := score - 10
		var want int
		if n < 0 && n%2 != 0 {
			want = n/2 - 1
		} else {
			want = n / 2
		}
		if got != want {
			rt.Fatalf("abilityModFrom(%d) = %d, want %d", score, got, want)
		}
	})
}

// TestApplySkillCheckEffect_Condition verifies that a "condition" effect type
// results in the named condition being applied to the session's ActiveSet.
//
// Precondition: condRegistry contains "distrusted"; sess.Conditions is initialized.
// Postcondition: sess.Conditions.Has("distrusted") returns true.
func TestApplySkillCheckEffect_Condition(t *testing.T) {
	logger := zaptest.NewLogger(t)

	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{
		ID:           "distrusted",
		Name:         "Distrusted",
		DurationType: "permanent",
		MaxStacks:    0,
	})

	svc := &GameServiceServer{
		condRegistry: reg,
		logger:       logger,
	}

	sess := &session.PlayerSession{
		UID:        "test_uid",
		Conditions: condition.NewActiveSet(),
	}

	svc.applySkillCheckEffect(sess, &skillcheck.Effect{Type: "condition", ID: "distrusted"}, "")

	assert.True(t, sess.Conditions.Has("distrusted"), "condition 'distrusted' must be active after apply")
}

// TestProperty_ApplySkillCheckEffect_ConditionAlwaysApplied verifies that:
//   - a condition that is registered is always applied after applySkillCheckEffect;
//   - a condition that is NOT registered is never applied.
//
// Precondition: id is an arbitrary valid condition ID string; register controls
// whether the ID is present in the registry before the call.
// Postcondition: sess.Conditions.Has(id) matches the value of register.
func TestProperty_ApplySkillCheckEffect_ConditionAlwaysApplied(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate an arbitrary condition ID.
		id := rapid.StringMatching(`[a-z_]{3,20}`).Draw(rt, "condition_id")
		// Decide randomly whether to register it.
		register := rapid.Bool().Draw(rt, "register")

		logger := zaptest.NewLogger(t)

		reg := condition.NewRegistry()
		if register {
			reg.Register(&condition.ConditionDef{
				ID:           id,
				Name:         id,
				DurationType: "permanent",
				MaxStacks:    0,
			})
		}

		svc := &GameServiceServer{
			condRegistry: reg,
			logger:       logger,
		}

		sess := &session.PlayerSession{
			UID:        "test_uid",
			Conditions: condition.NewActiveSet(),
		}

		svc.applySkillCheckEffect(sess, &skillcheck.Effect{Type: "condition", ID: id}, "")

		got := sess.Conditions.Has(id)
		if register && !got {
			rt.Fatalf("condition %q was registered but not applied; Has() = false", id)
		}
		if !register && got {
			rt.Fatalf("condition %q was NOT registered but was applied; Has() = true", id)
		}
	})
}

// TestApplySkillCheckEffect_Reveal_UnhidesExit verifies that a "reveal" effect
// un-hides the exit specified by effect.Target from the given room.
//
// Precondition: world contains room_a with a hidden north exit; effect.Target == "north".
// Postcondition: after applySkillCheckEffect, the north exit is no longer hidden.
func TestApplySkillCheckEffect_Reveal_UnhidesExit(t *testing.T) {
	logger := zaptest.NewLogger(t)

	zone := &world.Zone{
		ID:        "reveal_svc_test",
		Name:      "Reveal Svc Test",
		StartRoom: "room_a",
		Rooms: map[string]*world.Room{
			"room_a": {
				ID:          "room_a",
				ZoneID:      "reveal_svc_test",
				Title:       "Room A",
				Description: "Test room.",
				Exits: []world.Exit{
					{Direction: world.North, TargetRoom: "room_b", Hidden: true},
				},
				Properties: map[string]string{},
				MapX:        0,
				MapY:        0,
			},
			"room_b": {
				ID:          "room_b",
				ZoneID:      "reveal_svc_test",
				Title:       "Room B",
				Description: "North room.",
				Exits:       []world.Exit{{Direction: world.South, TargetRoom: "room_a"}},
				Properties:  map[string]string{},
				MapX:        0,
				MapY:        2,
			},
		},
	}
	mgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)

	svc := &GameServiceServer{
		world:  mgr,
		logger: logger,
	}

	sess := &session.PlayerSession{
		UID:    "test_uid",
		RoomID: "room_a",
	}

	svc.applySkillCheckEffect(sess, &skillcheck.Effect{Type: "reveal", Target: "north"}, "room_a")

	room, ok := mgr.GetRoom("room_a")
	require.True(t, ok)

	var northHidden bool
	for _, e := range room.Exits {
		if e.Direction == world.North {
			northHidden = e.Hidden
		}
	}
	assert.False(t, northHidden, "north exit must not be hidden after reveal effect")

	visible := room.VisibleExits()
	var foundNorth bool
	for _, e := range visible {
		if e.Direction == world.North {
			foundNorth = true
		}
	}
	assert.True(t, foundNorth, "north exit must appear in VisibleExits() after reveal effect")
}

// TestProperty_ApplySkillCheckEffect_Reveal is a property-based test (SWENG-5a) that
// exercises applySkillCheckEffect with the "reveal" type across all four cardinal
// directions and both hidden states.
//
// Preconditions: a world with room1 containing one exit in the drawn direction,
// hidden or not; a GameServiceServer backed by that world.
//
// Postconditions (invariants checked on every run):
//   - The visible exit count never decreases after a reveal effect.
//   - A previously hidden exit becomes visible (count increases by exactly 1).
//   - A previously visible exit count is unchanged.
func TestProperty_ApplySkillCheckEffect_Reveal(t *testing.T) {
	logger := zaptest.NewLogger(t)

	directions := []world.Direction{world.North, world.South, world.East, world.West}

	// oppositeDir returns the opposite direction so that room2's return exit is valid.
	oppositeDir := func(d world.Direction) world.Direction {
		switch d {
		case world.North:
			return world.South
		case world.South:
			return world.North
		case world.East:
			return world.West
		default: // West
			return world.East
		}
	}

	rapid.Check(t, func(rt *rapid.T) {
		dir := rapid.SampledFrom(directions).Draw(rt, "direction")
		hidden := rapid.Bool().Draw(rt, "hidden")

		zone := &world.Zone{
			ID:        "prop_reveal_zone",
			Name:      "Property Reveal Zone",
			StartRoom: "room1",
			Rooms: map[string]*world.Room{
				"room1": {
					ID:          "room1",
					ZoneID:      "prop_reveal_zone",
					Title:       "Room 1",
					Description: "A room.",
					Exits: []world.Exit{
						{Direction: dir, TargetRoom: "room2", Hidden: hidden},
					},
					Properties: map[string]string{},
					MapX:        0,
					MapY:        0,
				},
				"room2": {
					ID:          "room2",
					ZoneID:      "prop_reveal_zone",
					Title:       "Room 2",
					Description: "Another room.",
					Exits: []world.Exit{
						{Direction: oppositeDir(dir), TargetRoom: "room1"},
					},
					Properties: map[string]string{},
					MapX:        0,
					MapY:        2,
				},
			},
		}

		mgr, err := world.NewManager([]*world.Zone{zone})
		if err != nil {
			rt.Skip()
		}

		room1, ok := mgr.GetRoom("room1")
		if !ok {
			rt.Fatal("room1 not found in manager")
		}

		beforeVisible := len(room1.VisibleExits())

		svc := &GameServiceServer{
			world:  mgr,
			logger: logger,
		}
		sess := &session.PlayerSession{
			UID:        "prop_uid",
			RoomID:     "room1",
			Conditions: condition.NewActiveSet(),
		}

		svc.applySkillCheckEffect(sess, &skillcheck.Effect{Type: "reveal", Target: string(dir)}, "room1")

		room1after, ok := mgr.GetRoom("room1")
		if !ok {
			rt.Fatal("room1 not found after effect")
		}
		afterVisible := len(room1after.VisibleExits())

		// Invariant: visible count must never decrease.
		if afterVisible < beforeVisible {
			rt.Fatalf("visible exits decreased: direction=%s hidden=%v before=%d after=%d",
				dir, hidden, beforeVisible, afterVisible)
		}
		// If exit was hidden it must now be visible (count +1).
		if hidden && afterVisible != beforeVisible+1 {
			rt.Fatalf("hidden exit not revealed: direction=%s before=%d after=%d",
				dir, beforeVisible, afterVisible)
		}
		// If exit was already visible, count must be unchanged.
		if !hidden && afterVisible != beforeVisible {
			rt.Fatalf("visible exit count changed unexpectedly: direction=%s before=%d after=%d",
				dir, beforeVisible, afterVisible)
		}
	})
}

// TestHandleUse_AppliesConditionWhenConditionIDSet verifies that when an active feat has a non-empty
// ConditionID and the condRegistry contains that condition, calling handleUse applies the condition
// to the player session.
//
// Precondition: condRegistry contains "surge_active" (DurationType: "encounter", DamageBonus: 2).
// Player has feat "power-surge" with Active=true, ConditionID="surge_active".
// Postcondition: sess.Conditions.Has("surge_active") == true and response contains ActivateText.
func TestHandleUse_AppliesConditionWhenConditionIDSet(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:           "surge_active",
		Name:         "Power Surge",
		DurationType: "encounter",
		DamageBonus:  2,
	})

	feat := &ruleset.Feat{
		ID:           "power-surge",
		Name:         "Power Surge",
		Active:       true,
		ActivateText: "You surge with power!",
		ConditionID:  "surge_active",
	}
	featRegistry := ruleset.NewFeatRegistry([]*ruleset.Feat{feat})
	featsRepo := &stubFeatsRepo{
		data: map[int64][]string{
			0: {"power-surge"},
		},
	}

	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, nil, "",
		nil, nil, nil,
		[]*ruleset.Feat{feat}, featRegistry, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_use_cond",
		Username: "Hero",
		CharName: "Hero",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()

	event, err := svc.handleUse("u_use_cond", "power-surge")
	require.NoError(t, err)
	require.NotNil(t, event)

	useResp := event.GetUseResponse()
	require.NotNil(t, useResp, "expected UseResponse")
	assert.Equal(t, "You surge with power!", useResp.Message)

	assert.True(t, sess.Conditions.Has("surge_active"), "expected surge_active condition to be applied")
}

// TestHandleUse_NoConditionAppliedWhenConditionIDEmpty verifies that when an active feat has an
// empty ConditionID, calling handleUse does not apply any condition to the player session.
//
// Precondition: Player has feat "quick-strike" with Active=true, ConditionID="" (empty).
// Postcondition: sess.Conditions is empty (no conditions applied).
func TestHandleUse_NoConditionAppliedWhenConditionIDEmpty(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	feat := &ruleset.Feat{
		ID:           "quick-strike",
		Name:         "Quick Strike",
		Active:       true,
		ActivateText: "You strike quickly!",
		ConditionID:  "",
	}
	featRegistry := ruleset.NewFeatRegistry([]*ruleset.Feat{feat})
	featsRepo := &stubFeatsRepo{
		data: map[int64][]string{
			0: {"quick-strike"},
		},
	}

	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:           "some_condition",
		Name:         "Some Condition",
		DurationType: "encounter",
	})

	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, nil, "",
		nil, nil, nil,
		[]*ruleset.Feat{feat}, featRegistry, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_use_no_cond",
		Username: "Rogue",
		CharName: "Rogue",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()

	event, err := svc.handleUse("u_use_no_cond", "quick-strike")
	require.NoError(t, err)
	require.NotNil(t, event)

	useResp := event.GetUseResponse()
	require.NotNil(t, useResp, "expected UseResponse")
	assert.Equal(t, "You strike quickly!", useResp.Message)

	assert.Empty(t, sess.Conditions.All(), "expected no conditions applied when ConditionID is empty")
}

// TestProperty_HandleUse_ConditionIDEmpty_NoConditionApplied is a property test verifying that
// when a feat's ConditionID is empty, no condition is ever applied regardless of condRegistry contents.
//
// Precondition: feat has Active=true, ConditionID="" (always empty).
// Postcondition: sess.Conditions.All() is always empty after handleUse.
func TestProperty_HandleUse_ConditionIDEmpty_NoConditionApplied(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		condID := rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz_"))).Draw(rt, "condID")

		worldMgr, sessMgr := testWorldAndSession(t)
		logger := zaptest.NewLogger(t)

		feat := &ruleset.Feat{
			ID:           "prop-feat",
			Name:         "Prop Feat",
			Active:       true,
			ActivateText: "Activated.",
			ConditionID:  "", // always empty
		}
		featRegistry := ruleset.NewFeatRegistry([]*ruleset.Feat{feat})
		featsRepo := &stubFeatsRepo{
			data: map[int64][]string{
				0: {"prop-feat"},
			},
		}

		// Populate registry with condID so it exists but should never be applied.
		condReg := condition.NewRegistry()
		if condID != "" {
			condReg.Register(&condition.ConditionDef{
				ID:           condID,
				Name:         condID,
				DurationType: "encounter",
			})
		}

		svc := NewGameServiceServer(
			worldMgr, sessMgr,
			command.DefaultRegistry(),
			NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
			NewChatHandler(sessMgr),
			logger,
			nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil, condReg, nil, "",
			nil, nil, nil,
			[]*ruleset.Feat{feat}, featRegistry, featsRepo,
			nil, nil, nil, nil, nil, nil, nil,
			nil, nil,
		)

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:      "prop_uid_use",
			Username: "Tester",
			CharName: "Tester",
			RoomID:   "room_a",
			Role:     "player",
		})
		if err != nil {
			rt.Skip()
		}
		sess.Conditions = condition.NewActiveSet()

		_, err = svc.handleUse("prop_uid_use", "prop-feat")
		if err != nil {
			rt.Fatal("handleUse returned unexpected error:", err)
		}

		if len(sess.Conditions.All()) != 0 {
			rt.Fatalf("expected no conditions applied when ConditionID is empty, got %d conditions", len(sess.Conditions.All()))
		}
	})
}

func TestSkillDisplayName(t *testing.T) {
	cases := []struct {
		id   string
		want string
	}{
		{"parkour", "Parkour"},
		{"tech_lore", "Tech Lore"},
		{"hard_look", "Hard Look"},
		{"smooth_talk", "Smooth Talk"},
		{"rep", "Rep"},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			got := skillDisplayName(tc.id)
			assert.Equal(t, tc.want, got)
		})
	}
}

// newRaiseShieldSvc builds a minimal GameServiceServer for handleRaiseShield tests.
// condReg may be nil to test the no-registry path.
func newRaiseShieldSvc(t *testing.T, condReg *condition.Registry) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// TestHandleRaiseShield_NoSession verifies that handleRaiseShield returns an error when the
// player session does not exist.
//
// Precondition: uid "unknown_uid" has no session.
// Postcondition: error is returned; event is nil.
func TestHandleRaiseShield_NoSession(t *testing.T) {
	svc, _ := newRaiseShieldSvc(t, nil)
	event, err := svc.handleRaiseShield("unknown_uid")
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleRaiseShield_NoLoadout verifies that handleRaiseShield returns an error event when
// the player has no equipment loadout.
//
// Precondition: player session exists; sess.LoadoutSet == nil.
// Postcondition: error event with "no equipment" message; no error returned.
func TestHandleRaiseShield_NoLoadout(t *testing.T) {
	svc, sessMgr := newRaiseShieldSvc(t, nil)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_raiseshield_noloadout",
		Username: "Hero",
		CharName: "Hero",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.LoadoutSet = nil

	event, err := svc.handleRaiseShield("u_raiseshield_noloadout")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "no equipment")
}

// TestHandleRaiseShield_NoShield verifies that handleRaiseShield returns an error event when
// the player has no shield in the off-hand slot.
//
// Precondition: player has a loadout with no off-hand weapon.
// Postcondition: error event containing "shield"; no error returned.
func TestHandleRaiseShield_NoShield(t *testing.T) {
	svc, sessMgr := newRaiseShieldSvc(t, nil)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_raiseshield_noshield",
		Username: "Hero",
		CharName: "Hero",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	// LoadoutSet with empty preset (no off-hand weapon).
	sess.LoadoutSet = inventory.NewLoadoutSet()

	event, err := svc.handleRaiseShield("u_raiseshield_noshield")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "shield")
}

// TestHandleRaiseShield_OutOfCombat_ShieldEquipped verifies that handleRaiseShield applies the
// shield_raised condition and returns a success message when out of combat with a shield equipped.
//
// Precondition: condRegistry contains "shield_raised"; player has shield in off-hand; status is not in-combat.
// Postcondition: sess.Conditions.Has("shield_raised") == true; message event contains "+2 AC".
func TestHandleRaiseShield_OutOfCombat_ShieldEquipped(t *testing.T) {
	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:           "shield_raised",
		Name:         "Shield Raised",
		DurationType: "round",
	})

	svc, sessMgr := newRaiseShieldSvc(t, condReg)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_raiseshield_ooc",
		Username: "Hero",
		CharName: "Hero",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.LoadoutSet = inventory.NewLoadoutSet()
	shieldDef := &inventory.WeaponDef{
		ID:                  "wooden_shield",
		Name:                "Wooden Shield",
		Kind:                inventory.WeaponKindShield,
		DamageDice:          "1d4",
		DamageType:          "bludgeoning",
		ProficiencyCategory: "simple_weapons",
	}
	preset := sess.LoadoutSet.ActivePreset()
	require.NoError(t, preset.EquipOffHand(shieldDef))
	sess.Conditions = condition.NewActiveSet()
	// Status is default (out of combat).

	event, err := svc.handleRaiseShield("u_raiseshield_ooc")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "+2 AC")
	assert.True(t, sess.Conditions.Has("shield_raised"), "expected shield_raised condition applied")
}

// TestHandleRaiseShield_InCombat_InsufficientAP verifies that handleRaiseShield returns an error
// event when the player is in combat but has no active combat (SpendAP fails).
//
// Precondition: player status == statusInCombat; no CombatHandler set (combatH is nil).
// Postcondition: error event returned (SpendAP fails because combatH is nil/panics, or no combat found).
//
// Note: combatH is nil here so SpendAP is called on nil, which would panic.  We instead set
// Status to in-combat without a combatHandler to show the guard: because combatH is nil the
// service would panic if it called SpendAP.  To avoid requiring a real combat engine in unit
// tests, this test verifies the path by using a non-nil CombatHandler that has no registered
// combat (so SpendAP returns "not in active combat" error).
func TestHandleRaiseShield_InCombat_InsufficientAP(t *testing.T) {
	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:           "shield_raised",
		Name:         "Shield Raised",
		DurationType: "round",
	})

	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	combatHandler := NewCombatHandler(combat.NewEngine(), npc.NewManager(), sessMgr, nil, nil, 0, condReg, worldMgr, nil, nil, nil, nil, nil, nil)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_raiseshield_combat",
		Username: "Hero",
		CharName: "Hero",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.LoadoutSet = inventory.NewLoadoutSet()
	shieldDef := &inventory.WeaponDef{
		ID:                  "wooden_shield",
		Name:                "Wooden Shield",
		Kind:                inventory.WeaponKindShield,
		DamageDice:          "1d4",
		DamageType:          "bludgeoning",
		ProficiencyCategory: "simple_weapons",
	}
	preset := sess.LoadoutSet.ActivePreset()
	require.NoError(t, preset.EquipOffHand(shieldDef))
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat // triggers SpendAP path

	event, err := svc.handleRaiseShield("u_raiseshield_combat")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event when SpendAP fails (no active combat)")
}

// newTakeCoverSvc builds a minimal GameServiceServer for handleTakeCover tests.
// condReg may be nil to test the no-registry path.
func newTakeCoverSvc(t *testing.T, condReg *condition.Registry) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// TestHandleTakeCover_NoSession verifies that handleTakeCover returns an error when the
// player session does not exist.
//
// Precondition: uid "unknown_uid" has no session.
// Postcondition: error is returned; event is nil.
func TestHandleTakeCover_NoSession(t *testing.T) {
	svc, _ := newTakeCoverSvc(t, nil)
	event, err := svc.handleTakeCover("unknown_uid")
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleTakeCover_OutOfCombat verifies that handleTakeCover applies the standard_cover
// condition and returns a success message when out of combat (no AP cost).
//
// Precondition: condRegistry contains "standard_cover"; room has standard cover equipment;
// player status is not in-combat.
// Postcondition: sess.Conditions.Has("standard_cover") == true; message event contains "standard cover".
func TestHandleTakeCover_OutOfCombat(t *testing.T) {
	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:           "standard_cover",
		Name:         "Standard Cover",
		DurationType: "encounter",
		ACPenalty:    2,
		ReflexBonus:  2,
		StealthBonus: 2,
	})

	// Build a world with a room that has standard cover equipment.
	zone := &world.Zone{
		ID: "test_ooc_cover", Name: "Test", Description: "Test zone",
		StartRoom: "room_cover_ooc",
		Rooms: map[string]*world.Room{
			"room_cover_ooc": {
				ID: "room_cover_ooc", ZoneID: "test_ooc_cover",
				Title: "Cover Room", Description: "A room.", Properties: map[string]string{},
				Equipment: []world.RoomEquipmentConfig{
					{ItemID: "barrel_ooc", CoverTier: combat.CoverTierStandard},
				},
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	sessMgr := session.NewManager()
	logger := zaptest.NewLogger(t)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_takecover_ooc",
		Username: "Hero",
		CharName: "Hero",
		RoomID:   "room_cover_ooc",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	// Status is default (out of combat).

	event, err := svc.handleTakeCover("u_takecover_ooc")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "standard cover")
	assert.True(t, sess.Conditions.Has("standard_cover"), "expected standard_cover condition applied")
}

// TestHandleTakeCover_InCombat_InsufficientAP verifies that handleTakeCover returns an error
// event when the player is in combat but SpendAP fails due to no active combat being registered.
//
// Precondition: player status == statusInCombat; room has cover equipment; CombatHandler has no registered combat.
// Postcondition: error event returned (SpendAP fails because no active combat found).
func TestHandleTakeCover_InCombat_InsufficientAP(t *testing.T) {
	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:           "standard_cover",
		Name:         "Standard Cover",
		DurationType: "encounter",
		ACPenalty:    2,
		ReflexBonus:  2,
		StealthBonus: 2,
	})

	// Build a world with a room that has cover equipment.
	zone := &world.Zone{
		ID: "test_iap_cover", Name: "Test", Description: "Test zone",
		StartRoom: "room_cover_iap",
		Rooms: map[string]*world.Room{
			"room_cover_iap": {
				ID: "room_cover_iap", ZoneID: "test_iap_cover",
				Title: "Cover Room", Description: "A room.", Properties: map[string]string{},
				Equipment: []world.RoomEquipmentConfig{
					{ItemID: "barrel_iap", CoverTier: combat.CoverTierStandard},
				},
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	sessMgr := session.NewManager()
	logger := zaptest.NewLogger(t)
	combatHandler := NewCombatHandler(combat.NewEngine(), npc.NewManager(), sessMgr, nil, nil, 0, condReg, worldMgr, nil, nil, nil, nil, nil, nil)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_takecover_combat",
		Username: "Hero",
		CharName: "Hero",
		RoomID:   "room_cover_iap",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat // triggers SpendAP path

	event, err := svc.handleTakeCover("u_takecover_combat")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event when SpendAP fails (no active combat)")
}

func TestOutcomeDisplayName(t *testing.T) {
	assert.Equal(t, "critical success", outcomeDisplayName(skillcheck.CritSuccess))
	assert.Equal(t, "success", outcomeDisplayName(skillcheck.Success))
	assert.Equal(t, "failure", outcomeDisplayName(skillcheck.Failure))
	assert.Equal(t, "critical failure", outcomeDisplayName(skillcheck.CritFailure))
}

func TestPropertySkillDisplayName_NeverEmpty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.StringMatching(`[a-z][a-z_]{0,19}`).Draw(rt, "id")
		result := skillDisplayName(id)
		if result == "" {
			rt.Fatal("skillDisplayName must never return empty string")
		}
	})
}

// newFirstAidSvc builds a minimal GameServiceServer for handleFirstAid tests.
// roller may be nil to test the nil-dice path.
func newFirstAidSvc(t *testing.T, roller *dice.Roller, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, nil, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// TestHandleFirstAid_NoSession verifies that handleFirstAid returns an error when the
// player session does not exist.
//
// Precondition: uid "unknown_fa_uid" has no session.
// Postcondition: error is returned; event is nil.
func TestHandleFirstAid_NoSession(t *testing.T) {
	svc, _ := newFirstAidSvc(t, nil, nil)
	event, err := svc.handleFirstAid("unknown_fa_uid")
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleFirstAid_OutOfCombat_FailedCheck verifies that handleFirstAid returns a failure
// message when the skill check total is below DC 15.
//
// Precondition: player has no patch_job skill (bonus=0); dice source returns 0 (roll=1, total=1 < DC 15).
// Postcondition: message event contains "failure".
func TestHandleFirstAid_OutOfCombat_FailedCheck(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// Intn(20)=0 → roll=1, bonus=0, total=1 < DC 15 → failure.
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr := newFirstAidSvc(t, roller, nil)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_fa_fail",
		Username:  "Medic",
		CharName:  "Medic",
		RoomID:    "room_a",
		CurrentHP: 5,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	sess.Skills = map[string]string{} // no patch_job → bonus=0

	event, err := svc.handleFirstAid("u_fa_fail")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "failure")
}

// TestHandleFirstAid_OutOfCombat_SuccessCheck verifies that handleFirstAid heals the player
// when the skill check total meets or exceeds DC 15.
//
// Precondition: player has patch_job="trained" (bonus=2); dice source returns 12 (roll=13, total=15 >= DC 15).
// Postcondition: message event contains "success"; player CurrentHP increases.
func TestHandleFirstAid_OutOfCombat_SuccessCheck(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// Intn(20)=12 → roll=13; patch_job=trained → bonus=2; total=15 >= DC 15 → success.
	// Intn(8) for 2d8 also returns 0 each → each d8=1; +4 fixed → healed=6.
	src := &fixedDiceSource{val: 12}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr := newFirstAidSvc(t, roller, nil)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_fa_success",
		Username:  "Medic",
		CharName:  "Medic",
		RoomID:    "room_a",
		CurrentHP: 5,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	sess.Skills = map[string]string{"patch_job": "trained"}

	event, err := svc.handleFirstAid("u_fa_success")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "success")
	assert.Greater(t, sess.CurrentHP, 5, "expected HP to increase on success")
}

// TestHandleFirstAid_InCombat_InsufficientAP verifies that handleFirstAid returns an error
// event when the player is in combat but SpendAP fails (no active combat).
//
// Precondition: player status == statusInCombat; CombatHandler has no registered combat.
// Postcondition: error event returned.
func TestHandleFirstAid_InCombat_InsufficientAP(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	worldMgr, sessMgr := testWorldAndSession(t)
	combatHandler := NewCombatHandler(combat.NewEngine(), npc.NewManager(), sessMgr, nil, nil, 0, nil, worldMgr, nil, nil, nil, nil, nil, nil)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, nil, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_fa_combat",
		Username:  "Medic",
		CharName:  "Medic",
		RoomID:    "room_a",
		CurrentHP: 5,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat // triggers SpendAP path

	event, err := svc.handleFirstAid("u_fa_combat")
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event when SpendAP fails (no active combat)")
}

// newFeintSvc builds a minimal GameServiceServer for handleFeint tests.
// npcMgr may be nil; combatHandler may be nil.
func newFeintSvc(t *testing.T, roller *dice.Roller, npcMgr *npc.Manager, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	if npcMgr == nil {
		npcMgr = npc.NewManager()
	}
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// TestHandleFeint_NoSession verifies that handleFeint returns an error when the
// player session does not exist.
//
// Precondition: uid "unknown_feint_uid" has no session.
// Postcondition: error is returned; event is nil.
func TestHandleFeint_NoSession(t *testing.T) {
	svc, _ := newFeintSvc(t, nil, nil, nil)
	event, err := svc.handleFeint("unknown_feint_uid", &gamev1.FeintRequest{Target: "bandit"})
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleFeint_NotInCombat verifies that handleFeint returns an error event
// when the player is not in combat.
//
// Precondition: sess.Status != statusInCombat.
// Postcondition: error event containing "only available in combat".
func TestHandleFeint_NotInCombat(t *testing.T) {
	svc, sessMgr := newFeintSvc(t, nil, nil, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_feint_nc",
		Username: "Rogue",
		CharName: "Rogue",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleFeint("u_feint_nc", &gamev1.FeintRequest{Target: "bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleFeint_EmptyTarget verifies that handleFeint returns an error event
// when no target is specified.
//
// Precondition: player in combat; req.Target == "".
// Postcondition: error event containing "Usage: feint".
func TestHandleFeint_EmptyTarget(t *testing.T) {
	svc, sessMgr := newFeintSvc(t, nil, nil, nil)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_feint_et",
		Username: "Rogue",
		CharName: "Rogue",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	event, err := svc.handleFeint("u_feint_et", &gamev1.FeintRequest{Target: ""})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "Usage: feint")
}

// TestHandleFeint_InCombat_NoActiveSession verifies that handleFeint returns an error
// event when SpendAP fails due to no active combat registered.
//
// Precondition: player in combat; CombatHandler has no registered combat.
// Postcondition: error event returned from SpendAP failure.
func TestHandleFeint_InCombat_NoActiveSession(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	worldMgr, sessMgr := testWorldAndSession(t)
	combatHandler := NewCombatHandler(combat.NewEngine(), npc.NewManager(), sessMgr, nil, nil, 0, nil, worldMgr, nil, nil, nil, nil, nil, nil)
	npcMgr := npc.NewManager()
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_feint_nas",
		Username: "Rogue",
		CharName: "Rogue",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	event, err := svc.handleFeint("u_feint_nas", &gamev1.FeintRequest{Target: "bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event when SpendAP fails (no active combat)")
}

// newFeintSvcWithCombat builds a GameServiceServer, session.Manager, npc.Manager, and
// CombatHandler sharing the same session.Manager, suitable for tests that need real
// in-progress combat state.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc, sessMgr, npcMgr, and combatHandler all sharing the same sessMgr.
func newFeintSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleFeint_TargetNotFound verifies that handleFeint returns an error event when
// the named target NPC is not in the player's room, and that AP is NOT spent.
//
// Precondition: player in combat; no NPC named "ghost" in room.
// Postcondition: error event containing "not found"; AP is not decremented.
func TestHandleFeint_TargetNotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newFeintSvcWithCombat(t, roller)

	const roomID = "room_feint_tnf"
	// Spawn a real NPC so combat can start, but we'll feint against a non-existent target.
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-tnf", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_feint_tnf", Username: "Rogue", CharName: "Rogue",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	// Start combat by attacking so AP is allocated.
	_, err = combatHandler.Attack("u_feint_tnf", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	apBefore := combatHandler.RemainingAP("u_feint_tnf")

	event, err := svc.handleFeint("u_feint_tnf", &gamev1.FeintRequest{Target: "ghost"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected error event for non-existent target")
	assert.Contains(t, errEvt.Message, "not found")

	// AP must not have been spent.
	apAfter := combatHandler.RemainingAP("u_feint_tnf")
	assert.Equal(t, apBefore, apAfter, "AP must not be spent when target is not found")

	_ = sess
}

// TestHandleFeint_Success_RollBelow verifies that handleFeint returns a failure message
// when the grift roll total is below the target's Perception DC.
//
// Precondition: player in combat; NPC in room with Perception=15; dice returns 0 (roll=1, bonus=0, total=1 < 15).
// Postcondition: message event containing "failure".
func TestHandleFeint_Success_RollBelow(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// Intn(20)=0 → roll=1; no grift skill → bonus=0; total=1 < Perception=15 → failure.
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newFeintSvcWithCombat(t, roller)

	const roomID = "room_feint_rb"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "bandit-rb", Name: "Bandit", Level: 1, MaxHP: 20, AC: 13, Perception: 15,
	}, roomID)
	require.NoError(t, err)

	sessRB, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_feint_rb", Username: "Rogue", CharName: "Rogue",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sessRB.Status = statusInCombat

	_, err = combatHandler.Attack("u_feint_rb", "Bandit")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleFeint("u_feint_rb", &gamev1.FeintRequest{Target: "Bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on failed feint")
	assert.Contains(t, msgEvt.Content, "failure")
}

// TestHandleFeint_Success_RollAbove verifies that handleFeint returns a success message
// and calls ApplyCombatantACMod when the grift roll total meets or exceeds the Perception DC.
//
// Precondition: player in combat; NPC in room with Perception=5; dice returns 19 (roll=20, bonus=0, total=20 >= 5).
// Postcondition: message event containing "success"; NPC combatant ACMod is decremented.
func TestHandleFeint_Success_RollAbove(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// Intn(20)=19 → roll=20; no grift skill → bonus=0; total=20 >= Perception=5 → success.
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newFeintSvcWithCombat(t, roller)

	const roomID = "room_feint_ra"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-ra", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Perception: 5,
	}, roomID)
	require.NoError(t, err)

	sessRA, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_feint_ra", Username: "Rogue", CharName: "Rogue",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sessRA.Status = statusInCombat

	_, err = combatHandler.Attack("u_feint_ra", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleFeint("u_feint_ra", &gamev1.FeintRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on successful feint")
	assert.Contains(t, msgEvt.Content, "success")

	// Verify that ApplyCombatantACMod was called: the NPC combatant must have ACMod == -2.
	c, ok := combatHandler.GetCombatant("u_feint_ra", inst.ID)
	require.True(t, ok, "expected to find NPC combatant after feint")
	assert.Equal(t, -2, c.ACMod, "NPC ACMod must be -2 after successful feint")
}

// newDemoralizeSvc builds a minimal GameServiceServer for handleDemoralize tests.
// npcMgr may be nil; combatHandler may be nil.
func newDemoralizeSvc(t *testing.T, roller *dice.Roller, npcMgr *npc.Manager, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	if npcMgr == nil {
		npcMgr = npc.NewManager()
	}
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// newDemoralizeSvcWithCombat builds a GameServiceServer, session.Manager, npc.Manager, and
// CombatHandler sharing the same session.Manager, suitable for tests that need real
// in-progress combat state.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc, sessMgr, npcMgr, and combatHandler all sharing the same sessMgr.
func newDemoralizeSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleDemoralize_NoSession verifies that handleDemoralize returns an error when the
// player session does not exist.
//
// Precondition: uid "unknown_dem_uid" has no session.
// Postcondition: error is returned; event is nil.
func TestHandleDemoralize_NoSession(t *testing.T) {
	svc, _ := newDemoralizeSvc(t, nil, nil, nil)
	event, err := svc.handleDemoralize("unknown_dem_uid", &gamev1.DemoralizeRequest{Target: "bandit"})
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleDemoralize_NotInCombat verifies that handleDemoralize returns an error event
// when the player is not in combat.
//
// Precondition: sess.Status != statusInCombat.
// Postcondition: error event containing "only available in combat".
func TestHandleDemoralize_NotInCombat(t *testing.T) {
	svc, sessMgr := newDemoralizeSvc(t, nil, nil, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_dem_nc",
		Username: "Smooth",
		CharName: "Smooth",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleDemoralize("u_dem_nc", &gamev1.DemoralizeRequest{Target: "bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleDemoralize_EmptyTarget verifies that handleDemoralize returns an error event
// when no target is specified.
//
// Precondition: player in combat; req.Target == "".
// Postcondition: error event containing "Usage: demoralize".
func TestHandleDemoralize_EmptyTarget(t *testing.T) {
	svc, sessMgr := newDemoralizeSvc(t, nil, nil, nil)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_dem_et",
		Username: "Smooth",
		CharName: "Smooth",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	event, err := svc.handleDemoralize("u_dem_et", &gamev1.DemoralizeRequest{Target: ""})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "Usage: demoralize")
}

// TestHandleDemoralize_TargetNotFound verifies that handleDemoralize returns an error event
// when the named target NPC is not in the player's room, and that AP is NOT spent.
//
// Precondition: player in combat; no NPC named "ghost" in room.
// Postcondition: error event containing "not found"; AP is not decremented.
func TestHandleDemoralize_TargetNotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newDemoralizeSvcWithCombat(t, roller)

	const roomID = "room_dem_tnf"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-dem-tnf", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_dem_tnf", Username: "Smooth", CharName: "Smooth",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_dem_tnf", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	apBefore := combatHandler.RemainingAP("u_dem_tnf")

	event, err := svc.handleDemoralize("u_dem_tnf", &gamev1.DemoralizeRequest{Target: "ghost"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected error event for non-existent target")
	assert.Contains(t, errEvt.Message, "not found")

	apAfter := combatHandler.RemainingAP("u_dem_tnf")
	assert.Equal(t, apBefore, apAfter, "AP must not be spent when target is not found")

	_ = sess
}

// TestHandleDemoralize_RollBelow verifies that handleDemoralize returns a failure message
// when the smooth_talk roll total is below the target's Cool DC.
//
// Precondition: player in combat; NPC Level=5, Savvy=10 → Cool DC=15; dice returns 0 (roll=1, bonus=0, total=1 < 15).
// Postcondition: message event containing "failure".
func TestHandleDemoralize_RollBelow(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newDemoralizeSvcWithCombat(t, roller)

	const roomID = "room_dem_rb"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "bandit-dem-rb", Name: "Bandit", Level: 5, MaxHP: 20, AC: 13, Perception: 5,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)

	sessRB, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_dem_rb", Username: "Smooth", CharName: "Smooth",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sessRB.Status = statusInCombat

	_, err = combatHandler.Attack("u_dem_rb", "Bandit")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleDemoralize("u_dem_rb", &gamev1.DemoralizeRequest{Target: "Bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on failed demoralize")
	assert.Contains(t, msgEvt.Content, "failure")
}

// TestHandleDemoralize_RollAbove verifies that handleDemoralize returns a success message
// and applies -1 AC and -1 attack when the smooth_talk roll meets or exceeds the Cool DC.
//
// Precondition: player in combat; NPC Level=1, Savvy=10 → Cool DC=11; dice returns 19 (roll=20, bonus=0, total=20 >= 11).
// Postcondition: message event containing "success".
func TestHandleDemoralize_RollAbove(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newDemoralizeSvcWithCombat(t, roller)

	const roomID = "room_dem_ra"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-dem-ra", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Perception: 5,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)

	sessRA, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_dem_ra", Username: "Smooth", CharName: "Smooth",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sessRA.Status = statusInCombat

	_, err = combatHandler.Attack("u_dem_ra", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleDemoralize("u_dem_ra", &gamev1.DemoralizeRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on successful demoralize")
	assert.Contains(t, msgEvt.Content, "success")

	// Verify that ApplyCombatantACMod and ApplyCombatantAttackMod were both called:
	// the NPC combatant must have ACMod == -1 and AttackMod == -1.
	c, ok := combatHandler.GetCombatant("u_dem_ra", inst.ID)
	require.True(t, ok, "expected to find NPC combatant after demoralize")
	assert.Equal(t, -1, c.ACMod, "NPC ACMod must be -1 after successful demoralize")
	assert.Equal(t, -1, c.AttackMod, "NPC AttackMod must be -1 after successful demoralize")
}

func TestHandleChar_Awareness_TrainedRank(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	_ = worldMgr
	uid := "player-uid-awareness"
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "tester", CharName: "Tester",
		RoomID: "grinders_row", Role: "player", CharacterID: 0,
		CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Abilities.Savvy = 14 // mod = +2
	sess.Level = 1
	sess.Proficiencies = map[string]string{"awareness": "trained"}

	svc := testMinimalService(t, sessMgr)
	evt, err := svc.handleChar(uid)
	require.NoError(t, err)
	sheet := evt.GetCharacterSheet()
	require.NotNil(t, sheet)
	// 10 + AbilityMod(14)=+2 + CombatProficiencyBonus(1,"trained")=3 = 15
	assert.Equal(t, int32(15), sheet.GetAwareness())
}

func TestHandleChar_Awareness_UntrainedRank(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	_ = worldMgr
	uid := "player-uid-awareness-2"
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "tester2", CharName: "Tester2",
		RoomID: "grinders_row", Role: "player", CharacterID: 0,
		CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Abilities.Savvy = 10
	sess.Level = 1
	sess.Proficiencies = map[string]string{"awareness": "untrained"}

	svc := testMinimalService(t, sessMgr)
	evt, err := svc.handleChar(uid)
	require.NoError(t, err)
	sheet := evt.GetCharacterSheet()
	require.NotNil(t, sheet)
	// 10 + AbilityMod(10)=0 + CombatProficiencyBonus(1,"untrained")=0 = 10
	assert.Equal(t, int32(10), sheet.GetAwareness())
}

func TestHandleChar_Awareness_BackfilledWhenMissing(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	_ = worldMgr
	uid := "player-uid-awareness-bf"
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "bf", CharName: "BF",
		RoomID: "grinders_row", Role: "player", CharacterID: 0,
		CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Abilities.Savvy = 10
	sess.Level = 1
	sess.Proficiencies = map[string]string{} // no awareness

	svc := testMinimalService(t, sessMgr)
	evt, err := svc.handleChar(uid)
	require.NoError(t, err)
	sheet := evt.GetCharacterSheet()
	// With trained backfill: 10 + AbilityMod(10)=0 + CombatProficiencyBonus(1,"trained")=3 = 13
	assert.Equal(t, int32(13), sheet.GetAwareness())
}

// drainEntity reads all buffered events from entity without blocking.
func drainEntity(e *session.BridgeEntity) {
	for {
		select {
		case <-e.Events():
		default:
			return
		}
	}
}

func TestPushRoomViewToAllInRoom_SendsToPlayersInRoom(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	_ = worldMgr
	uid := "rv-player-1"
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "rv1", CharName: "RV1",
		RoomID: "room_a", Role: "player", CharacterID: 0,
		CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)

	svc := testMinimalService(t, sessMgr)
	drainEntity(sess.Entity)

	svc.pushRoomViewToAllInRoom("room_a")

	select {
	case data := <-sess.Entity.Events():
		require.NotNil(t, data)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected RoomView event within 500ms")
	}
}

func TestPushRoomViewToAllInRoom_SkipsPlayersElsewhere(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	_ = worldMgr
	uid := "rv-player-2"
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "rv2", CharName: "RV2",
		RoomID: "room_a", Role: "player", CharacterID: 0,
		CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	drainEntity(sess.Entity)

	svc := testMinimalService(t, sessMgr)

	svc.pushRoomViewToAllInRoom("nonexistent_room")

	select {
	case <-sess.Entity.Events():
		t.Fatal("player in different room should not receive RoomView")
	case <-time.After(200 * time.Millisecond):
		// correct: nothing received
	}
}

// TestProperty_HandleDemoralize_CoolDC_Formula verifies that the Cool DC
// used by handleDemoralize equals 10 + level + abilityMod(savvy) + rankBonus.
//
// Precondition: rapid generates level (1-20), savvy (1-20), rank string.
// Postcondition: message content contains the expected DC value.
func TestProperty_HandleDemoralize_CoolDC_Formula(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		savvy := rapid.IntRange(1, 20).Draw(rt, "savvy")
		rank := rapid.SampledFrom([]string{"", "trained", "expert", "master", "legendary"}).Draw(rt, "rank")

		expectedMod := combat.AbilityMod(savvy)
		expectedRankBonus := skillRankBonus(rank)
		expectedDC := 10 + level + expectedMod + expectedRankBonus

		tmpl := &npc.Template{
			ID: fmt.Sprintf("dem-prop-%d-%d", level, savvy), Name: "Target", Level: level,
			MaxHP: 20, AC: 13, Perception: 5,
			Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: savvy},
			CoolRank:  rank,
		}

		logger := zaptest.NewLogger(t)
		src := &fixedDiceSource{val: 99}
		roller := dice.NewLoggedRoller(src, logger)
		svc, sessMgr, npcMgr, combatHandler := newDemoralizeSvcWithCombat(t, roller)

		roomID := fmt.Sprintf("room_dem_prop_%d_%d", level, savvy)
		uid := fmt.Sprintf("u_dem_prop_%d_%d", level, savvy)
		_, err := npcMgr.Spawn(tmpl, roomID)
		require.NoError(rt, err)
		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: uid, Username: "F", CharName: "F",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		require.NoError(rt, err)
		sess.Status = statusInCombat
		_, err = combatHandler.Attack(uid, "Target")
		require.NoError(rt, err)
		combatHandler.cancelTimer(roomID)

		event, err := svc.handleDemoralize(uid, &gamev1.DemoralizeRequest{Target: "Target"})
		require.NoError(rt, err)
		require.NotNil(rt, event)
		msgEvt := event.GetMessage()
		require.NotNil(rt, msgEvt)
		assert.Contains(rt, msgEvt.Content, fmt.Sprintf("DC %d", expectedDC),
			"message must include computed Cool DC")
	})
}
