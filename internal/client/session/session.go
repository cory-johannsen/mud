// internal/client/session/session.go
package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// SessionState represents the client session state machine state.
type SessionState int

const (
	StateDisconnected    SessionState = iota
	StateAuthenticating
	StateCharacterSelect
	StateInGame
	StateReconnecting
)

// CharacterState is the current character panel data, updated automatically by the recv pump.
type CharacterState struct {
	Name       string
	Level      int
	CurrentHP  int
	MaxHP      int
	Conditions []string
	HeroPoints int
	AP         int
}

// State is a snapshot of the session.
type State struct {
	Current   SessionState
	Character *CharacterState // non-nil when StateInGame
	Error     error           // last terminal error
}

// Session manages a gRPC GameService.Session stream and the client state machine.
type Session struct {
	grpcAddr  string
	cmdParser func(string) (*gamev1.ClientMessage, error)
	conn      *grpc.ClientConn // nil when using grpcAddr, set by NewWithConn

	mu        sync.RWMutex
	state     SessionState
	charState *CharacterState
	lastErr   error

	stream   gamev1.GameService_SessionClient
	cancelFn context.CancelFunc
	events   chan *gamev1.ServerEvent
	sendMu   sync.Mutex
}

// New creates a Session that dials grpcAddr on Connect.
func New(grpcAddr string, cmdParser func(string) (*gamev1.ClientMessage, error)) *Session {
	return &Session{
		grpcAddr:  grpcAddr,
		cmdParser: cmdParser,
		state:     StateDisconnected,
		events:    make(chan *gamev1.ServerEvent, 64),
	}
}

// NewWithConn creates a Session using an existing gRPC connection (used in tests).
func NewWithConn(conn *grpc.ClientConn, cmdParser func(string) (*gamev1.ClientMessage, error)) *Session {
	return &Session{
		conn:      conn,
		cmdParser: cmdParser,
		state:     StateDisconnected,
		events:    make(chan *gamev1.ServerEvent, 64),
	}
}

// Connect opens the gRPC stream and transitions to StateInGame.
// Returns an error if already connected.
func (s *Session) Connect(jwt string, characterID int64) error {
	s.mu.Lock()
	if s.state != StateDisconnected {
		s.mu.Unlock()
		return fmt.Errorf("session already connected (state=%d)", s.state)
	}
	s.state = StateAuthenticating
	s.mu.Unlock()

	conn, err := s.dial()
	if err != nil {
		s.transitionDisconnected(err)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	client := gamev1.NewGameServiceClient(conn)
	stream, err := client.Session(ctx)
	if err != nil {
		cancel()
		s.transitionDisconnected(err)
		return err
	}

	// Send JoinWorldRequest
	joinMsg := &gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_JoinWorld{
			JoinWorld: &gamev1.JoinWorldRequest{
				CharacterId: characterID,
			},
		},
	}
	if err := stream.Send(joinMsg); err != nil {
		cancel()
		s.transitionDisconnected(err)
		return err
	}

	s.mu.Lock()
	s.stream = stream
	s.cancelFn = cancel
	s.state = StateInGame
	s.mu.Unlock()

	go s.recvPump()
	return nil
}

// Send parses cmd and sends it over the gRPC stream.
func (s *Session) Send(cmd string) error {
	msg, err := s.cmdParser(cmd)
	if err != nil {
		return fmt.Errorf("parse command: %w", err)
	}
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	s.mu.RLock()
	stream := s.stream
	s.mu.RUnlock()
	if stream == nil {
		return fmt.Errorf("not connected")
	}
	return stream.Send(msg)
}

// Events returns the channel of ServerEvents from the gRPC stream.
// The channel is closed when the session transitions to StateDisconnected.
func (s *Session) Events() <-chan *gamev1.ServerEvent {
	return s.events
}

// State returns a snapshot of the current session state.
func (s *Session) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return State{
		Current:   s.state,
		Character: s.charState,
		Error:     s.lastErr,
	}
}

// Close gracefully shuts down the gRPC stream.
func (s *Session) Close() error {
	s.mu.Lock()
	cancel := s.cancelFn
	stream := s.stream
	s.state = StateDisconnected
	s.stream = nil
	s.cancelFn = nil
	s.mu.Unlock()

	var err error
	if stream != nil {
		err = stream.CloseSend()
	}
	if cancel != nil {
		cancel()
	}
	return err
}

// dial returns the gRPC connection, dialing if needed.
func (s *Session) dial() (*grpc.ClientConn, error) {
	if s.conn != nil {
		return s.conn, nil
	}
	return grpc.NewClient(s.grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
}

// recvPump reads ServerEvents and publishes them to the events channel.
// It also updates CharacterState from CharacterInfo and CharacterSheet events.
func (s *Session) recvPump() {
	for {
		s.mu.RLock()
		stream := s.stream
		s.mu.RUnlock()
		if stream == nil {
			return
		}
		ev, err := stream.Recv()
		if err != nil {
			s.transitionDisconnected(err)
			close(s.events)
			return
		}
		s.updateCharacterState(ev)
		select {
		case s.events <- ev:
		default:
			// Drop if channel is full to avoid blocking the recv pump.
		}
	}
}

// updateCharacterState updates the stored CharacterState from CharacterInfo
// or CharacterSheetView events. Called from the recv pump.
func (s *Session) updateCharacterState(ev *gamev1.ServerEvent) {
	var cs *CharacterState
	switch p := ev.Payload.(type) {
	case *gamev1.ServerEvent_CharacterInfo:
		if p.CharacterInfo != nil {
			ci := p.CharacterInfo
			cs = &CharacterState{
				Name:      ci.GetName(),
				Level:     int(ci.GetLevel()),
				CurrentHP: int(ci.GetCurrentHp()),
				MaxHP:     int(ci.GetMaxHp()),
			}
		}
	case *gamev1.ServerEvent_CharacterSheet:
		if p.CharacterSheet != nil {
			sheet := p.CharacterSheet
			cs = &CharacterState{
				Name:       sheet.GetName(),
				Level:      int(sheet.GetLevel()),
				CurrentHP:  int(sheet.GetCurrentHp()),
				MaxHP:      int(sheet.GetMaxHp()),
				HeroPoints: int(sheet.GetHeroPoints()),
			}
		}
	}
	if cs != nil {
		s.mu.Lock()
		s.charState = cs
		s.mu.Unlock()
	}
}

func (s *Session) transitionDisconnected(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = StateDisconnected
	s.lastErr = err
	s.stream = nil
}

// reconnectBackoff attempts to reconnect with exponential backoff.
// Attempts: 2s, 4s, 8s. Transitions to StateDisconnected after the third failure.
// This is called by higher-level client code when the events channel is closed unexpectedly.
func (s *Session) reconnectBackoff(jwt string, characterID int64) {
	delays := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}
	for _, d := range delays {
		time.Sleep(d)
		s.mu.Lock()
		s.state = StateReconnecting
		s.mu.Unlock()
		s.events = make(chan *gamev1.ServerEvent, 64)
		if err := s.Connect(jwt, characterID); err == nil {
			return
		}
	}
	s.transitionDisconnected(fmt.Errorf("reconnect failed after %d attempts", len(delays)))
}
