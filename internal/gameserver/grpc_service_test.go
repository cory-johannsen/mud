package gameserver

import (
	"context"
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
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
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
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil,
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
	require.Len(t, msgs, 1, "expected exactly one outcome message")
	assert.Equal(t, "You vault it.", msgs[0])
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
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil,
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
	require.Len(t, msgs, 1)
	assert.Equal(t, "You stumble.", msgs[0])
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
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil,
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
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, nil, nil, scriptMgr,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil,
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
	require.Len(t, msgs, 1)
	assert.Equal(t, "You vault it.", msgs[0])

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
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, nil, nil, scriptMgr2,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil,
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
	require.Len(t, msgs2, 1)
	assert.Equal(t, "You vault it.", msgs2[0])

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
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil,
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
	require.Len(t, msgs, 1)
	assert.Equal(t, "You stumble and take damage.", msgs[0])

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
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, npcHandler, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil,
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
	require.Len(t, msgs, 1, "expected exactly one outcome message from NPC on_greet check")
	assert.Equal(t, "They warm up to you.", msgs[0])
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
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, npcHandler, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil,
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
	require.Len(t, msgs, 1, "expected exactly one outcome message from NPC on_greet check")
	assert.Equal(t, "They sneer.", msgs[0])
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
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, npcHandler, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil,
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
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, npcHandler, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil,
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
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, npcHandler, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil,
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
	require.Len(t, msgs, 1, "expected exactly one outcome message")
	assert.Equal(t, "They strike you!", msgs[0])

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
