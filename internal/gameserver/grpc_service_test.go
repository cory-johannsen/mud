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
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// testGRPCServer starts an in-process gRPC server and returns a connected client.
func testGRPCServer(t *testing.T) (gamev1.GameServiceClient, *session.Manager) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npc.NewManager())
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	svc := NewGameServiceServer(worldMgr, sessMgr, cmdRegistry, worldHandler, chatHandler, logger, nil, nil, nil, nil, nil)

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
	assert.Contains(t, playerList.Players, "Alice")
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
