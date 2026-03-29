// internal/client/session/session_test.go
package session_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/client/session"
	"github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// fakeGameServer implements gamev1.GameServiceServer minimally for testing.
type fakeGameServer struct {
	gamev1.UnimplementedGameServiceServer
	recvCh chan *gamev1.ClientMessage
	sendCh chan *gamev1.ServerEvent
}

func (f *fakeGameServer) Session(stream gamev1.GameService_SessionServer) error {
	// Send one event then block until client closes.
	for ev := range f.sendCh {
		if err := stream.Send(ev); err != nil {
			return err
		}
	}
	// Block until stream closes.
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if f.recvCh != nil {
			select {
			case f.recvCh <- msg:
			default:
			}
		}
	}
}

func newTestSession(t *testing.T, fake *fakeGameServer) (*session.Session, func()) {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	gamev1.RegisterGameServiceServer(srv, fake)
	go srv.Serve(lis)

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	parser := func(cmd string) (*gamev1.ClientMessage, error) {
		return &gamev1.ClientMessage{}, nil
	}

	s := session.NewWithConn(conn, parser)
	return s, func() {
		conn.Close()
		srv.Stop()
		lis.Close()
	}
}

func TestSession_InitialState(t *testing.T) {
	s := session.New("localhost:50051", func(string) (*gamev1.ClientMessage, error) {
		return &gamev1.ClientMessage{}, nil
	})
	state := s.State()
	assert.Equal(t, session.StateDisconnected, state.Current)
	assert.Nil(t, state.Character)
	assert.NoError(t, state.Error)
}

func TestSession_Connect_TransitionsToInGame(t *testing.T) {
	evCh := make(chan *gamev1.ServerEvent, 1)
	fake := &fakeGameServer{sendCh: evCh}
	s, cleanup := newTestSession(t, fake)
	defer cleanup()

	err := s.Connect("jwt.token", 42)
	require.NoError(t, err)
	assert.Equal(t, session.StateInGame, s.State().Current)
}

func TestSession_RecvEvent(t *testing.T) {
	ev := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Error{Error: &gamev1.ErrorEvent{Message: "hello"}},
	}
	evCh := make(chan *gamev1.ServerEvent, 1)
	evCh <- ev
	close(evCh)
	fake := &fakeGameServer{sendCh: evCh}
	s, cleanup := newTestSession(t, fake)
	defer cleanup()

	require.NoError(t, s.Connect("jwt", 1))

	select {
	case got := <-s.Events():
		require.NotNil(t, got)
		assert.Equal(t, "hello", got.GetError().GetMessage())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestSession_Send(t *testing.T) {
	recvCh := make(chan *gamev1.ClientMessage, 1)
	fake := &fakeGameServer{
		sendCh: make(chan *gamev1.ServerEvent),
		recvCh: recvCh,
	}
	close(fake.sendCh)
	s, cleanup := newTestSession(t, fake)
	defer cleanup()

	require.NoError(t, s.Connect("jwt", 1))
	require.NoError(t, s.Send("move north"))

	select {
	case msg := <-recvCh:
		assert.NotNil(t, msg)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive message")
	}
}

func TestSession_Close(t *testing.T) {
	fake := &fakeGameServer{sendCh: make(chan *gamev1.ServerEvent)}
	close(fake.sendCh)
	s, cleanup := newTestSession(t, fake)
	defer cleanup()

	require.NoError(t, s.Connect("jwt", 1))
	require.NoError(t, s.Close())
	assert.Equal(t, session.StateDisconnected, s.State().Current)
}

func TestSession_CharacterStateUpdatedFromCharacterInfo(t *testing.T) {
	ci := &gamev1.CharacterInfo{
		Name: "Zara", Level: 3, CurrentHp: 25, MaxHp: 30,
	}
	ev := &gamev1.ServerEvent{Payload: &gamev1.ServerEvent_CharacterInfo{CharacterInfo: ci}}
	evCh := make(chan *gamev1.ServerEvent, 1)
	evCh <- ev
	close(evCh)
	fake := &fakeGameServer{sendCh: evCh}
	s, cleanup := newTestSession(t, fake)
	defer cleanup()

	require.NoError(t, s.Connect("jwt", 1))
	// Drain the event
	<-s.Events()
	time.Sleep(10 * time.Millisecond)

	state := s.State()
	require.NotNil(t, state.Character)
	assert.Equal(t, "Zara", state.Character.Name)
	assert.Equal(t, 3, state.Character.Level)
	assert.Equal(t, 25, state.Character.CurrentHP)
	assert.Equal(t, 30, state.Character.MaxHP)
}
