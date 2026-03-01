package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// ErrSwitchCharacter is returned by gameBridge when the player uses the switch command.
// characterFlow checks for this sentinel to loop back to character selection.
var ErrSwitchCharacter = errors.New("switch character")

// IdleMonitorConfig configures the idle monitor goroutine.
type IdleMonitorConfig struct {
	// LastInput is the shared atomic timestamp (UnixNano) of the most recent player input.
	LastInput *atomic.Int64
	// IdleTimeout is the duration of silence before the warning callback fires.
	IdleTimeout time.Duration
	// GracePeriod is the duration after the warning before the disconnect callback fires.
	GracePeriod time.Duration
	// TickInterval controls how often the monitor checks for idleness.
	TickInterval time.Duration
	// OnWarning is called once when the player has been idle for IdleTimeout.
	OnWarning func()
	// OnDisconnect is called once when the player has been idle for IdleTimeout + GracePeriod.
	OnDisconnect func()
}

// StartIdleMonitor launches a goroutine that monitors player inactivity.
// It returns a stop function that terminates the goroutine cleanly.
//
// Precondition: cfg.LastInput must be non-nil; cfg.OnWarning and cfg.OnDisconnect must be non-nil.
// Postcondition: The goroutine exits when stop() is called or OnDisconnect fires.
func StartIdleMonitor(cfg IdleMonitorConfig) (stop func()) {
	done := make(chan struct{})
	var once sync.Once
	stop = func() { once.Do(func() { close(done) }) }

	go func() {
		ticker := time.NewTicker(cfg.TickInterval)
		defer ticker.Stop()
		warningSent := false
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				idle := time.Duration(time.Now().UnixNano() - cfg.LastInput.Load())
				if !warningSent && idle >= cfg.IdleTimeout {
					warningSent = true
					cfg.OnWarning()
				}
				if warningSent && idle >= cfg.IdleTimeout+cfg.GracePeriod {
					cfg.OnDisconnect()
					return
				}
			}
		}
	}()

	return stop
}

// gameBridge manages the gRPC session between a Telnet client and the game server.
//
// Precondition: conn must be open; char must be non-nil with a valid ID.
// Postcondition: Returns nil on clean disconnect, or a non-nil error on fatal failure.
func (h *AuthHandler) gameBridge(ctx context.Context, conn *telnet.Conn, acct postgres.Account, char *character.Character) error {
	sessionStart := time.Now()

	// Connect to gameserver
	grpcConn, err := grpc.NewClient(h.gameServerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		h.logger.Error("connecting to game server", zap.Error(err))
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Failed to connect to game server. Please try again later."))
		return fmt.Errorf("dialing game server: %w", err)
	}
	defer grpcConn.Close()

	client := gamev1.NewGameServiceClient(grpcConn)

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := client.Session(streamCtx)
	if err != nil {
		h.logger.Error("opening game session", zap.Error(err))
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Failed to start game session."))
		return fmt.Errorf("opening session stream: %w", err)
	}

	// Send JoinWorldRequest
	uid := fmt.Sprintf("%d", char.ID)
	if err := stream.Send(&gamev1.ClientMessage{
		RequestId: "join",
		Payload: &gamev1.ClientMessage_JoinWorld{
			JoinWorld: &gamev1.JoinWorldRequest{
				Uid:           uid,
				Username:      acct.Username,
				CharacterId:   char.ID,
				CharacterName: char.Name,
				CurrentHp:     int32(char.CurrentHP),
				Location:      char.Location,
				Role:          acct.Role,
				RegionDisplay: h.regionDisplayName(char.Region),
				Class:         char.Class,
				Level:         int32(char.Level),
			},
		},
	}); err != nil {
		return fmt.Errorf("sending join request: %w", err)
	}

	// Receive initial room view
	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receiving initial room view: %w", err)
	}
	if rv := resp.GetRoomView(); rv != nil {
		_ = conn.Write([]byte(RenderRoomView(rv)))
	}

	// Write initial prompt
	prompt := telnet.Colorf(telnet.BrightCyan, "[%s]> ", char.Name)
	if err := conn.WritePrompt(prompt); err != nil {
		return fmt.Errorf("writing initial prompt: %w", err)
	}

	var lastInput atomic.Int64
	lastInput.Store(time.Now().UnixNano())
	var disconnectReason atomic.Value
	disconnectReason.Store("quit")

	// currentRoom tracks the live room ID for the session.
	// It is initialized to the character's saved location and updated whenever
	// the server sends a RoomView event (i.e. after every move or look).
	var currentRoom atomic.Value
	currentRoom.Store(char.Location)

	stopIdle := StartIdleMonitor(IdleMonitorConfig{
		LastInput:    &lastInput,
		IdleTimeout:  h.telnetCfg.IdleTimeout,
		GracePeriod:  h.telnetCfg.IdleGracePeriod,
		TickInterval: 30 * time.Second,
		OnWarning: func() {
			msg := fmt.Sprintf(
				"Warning: You have been idle for %s. You will be disconnected in %s.",
				h.telnetCfg.IdleTimeout.Round(time.Second),
				h.telnetCfg.IdleGracePeriod.Round(time.Second),
			)
			if err := conn.WriteLine(telnet.Colorize(telnet.Yellow, msg)); err != nil {
				h.logger.Debug("failed to send idle warning", zap.Error(err))
			}
		},
		OnDisconnect: func() {
			disconnectReason.Store("inactivity")
			cancel()
		},
	})
	defer stopIdle()

	// Spawn goroutine to forward server events to Telnet.
	// After each event is rendered, the prompt is re-displayed.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.forwardServerEvents(streamCtx, stream, conn, char.Name, &currentRoom)
	}()

	// Command loop: read Telnet → parse → send gRPC
	err = h.commandLoop(streamCtx, stream, conn, char.Name, acct.Role, &lastInput)

	cancel()
	wg.Wait()
	stopIdle()

	if err != nil && !errors.Is(err, context.Canceled) && disconnectReason.Load().(string) == "quit" {
		disconnectReason.Store("connection_error")
	}
	if errors.Is(err, ErrSwitchCharacter) {
		disconnectReason.Store("switch_character")
	}

	h.logger.Info("player disconnected",
		zap.String("reason", disconnectReason.Load().(string)),
		zap.String("player", char.Name),
		zap.String("account", acct.Username),
		zap.Duration("session_duration", time.Since(sessionStart)),
		zap.String("room_id", currentRoom.Load().(string)),
	)

	if errors.Is(err, ErrSwitchCharacter) {
		return ErrSwitchCharacter
	}
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// commandLoop reads lines from the Telnet connection, parses commands,
// and sends them as gRPC ClientMessages.
//
// Precondition: stream must be open; charName must be non-empty; lastInput must be non-nil.
// Postcondition: Returns nil on clean quit, ctx.Err() on cancellation, or a wrapped error on failure.
func (h *AuthHandler) commandLoop(ctx context.Context, stream gamev1.GameService_SessionClient, conn *telnet.Conn, charName string, role string, lastInput *atomic.Int64) error {
	registry := command.DefaultRegistry()
	requestID := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := conn.ReadLine()
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		lastInput.Store(time.Now().UnixNano())

		line = strings.TrimSpace(line)
		if line == "" {
			_ = conn.WritePrompt(telnet.Colorf(telnet.BrightCyan, "[%s]> ", charName))
			continue
		}

		parsed := command.Parse(line)
		requestID++
		reqID := fmt.Sprintf("req-%d", requestID)

		cmd, ok := registry.Resolve(parsed.Command)
		if !ok {
			// Try it as a custom exit name (e.g., "stairs")
			msg := buildMoveMessage(reqID, parsed.Command)
			if err := stream.Send(msg); err != nil {
				return fmt.Errorf("sending message: %w", err)
			}
			continue
		}

		bctx := &bridgeContext{
			reqID:    reqID,
			cmd:      cmd,
			parsed:   parsed,
			conn:     conn,
			charName: charName,
			role:     role,
			stream:   stream,
			helpFn: func() {
				h.showGameHelp(conn, registry, role)
				_ = conn.WritePrompt(telnet.Colorf(telnet.BrightCyan, "[%s]> ", charName))
			},
		}

		handlerFn, ok := bridgeHandlerMap[cmd.Handler]
		if !ok {
			_ = conn.WriteLine(telnet.Colorf(telnet.Dim, "You don't know how to '%s'.", parsed.Command))
			_ = conn.WritePrompt(telnet.Colorf(telnet.BrightCyan, "[%s]> ", charName))
			continue
		}

		result, err := handlerFn(bctx)
		if err != nil {
			return err
		}
		if result.quit {
			return nil
		}
		if result.switchCharacter {
			if result.msg != nil {
				if err := stream.Send(result.msg); err != nil {
					return fmt.Errorf("sending switch request: %w", err)
				}
			}
			return ErrSwitchCharacter
		}
		if result.done {
			continue
		}
		if result.msg != nil {
			if err := stream.Send(result.msg); err != nil {
				return fmt.Errorf("sending message: %w", err)
			}
		}
	}
}

func buildMoveMessage(reqID, direction string) *gamev1.ClientMessage {
	return &gamev1.ClientMessage{
		RequestId: reqID,
		Payload: &gamev1.ClientMessage_Move{
			Move: &gamev1.MoveRequest{Direction: direction},
		},
	}
}

// forwardServerEvents reads ServerEvents from the gRPC stream and writes
// rendered text to the Telnet connection.
//
// Precondition: stream must be open; charName must be non-empty; currentRoom must be non-nil and hold a string.
// Postcondition: Returns when ctx is done, stream closes, or a disconnect event is received.
// Side-effect: currentRoom is updated to the latest RoomView.RoomId whenever a RoomView event is received.
func (h *AuthHandler) forwardServerEvents(ctx context.Context, stream gamev1.GameService_SessionClient, conn *telnet.Conn, charName string, currentRoom *atomic.Value) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		resp, err := stream.Recv()
		if err != nil {
			if err != io.EOF && ctx.Err() == nil {
				h.logger.Debug("stream recv error in forwarder", zap.Error(err))
			}
			return
		}

		var text string
		switch p := resp.Payload.(type) {
		case *gamev1.ServerEvent_RoomView:
			if roomID := p.RoomView.GetRoomId(); roomID != "" {
				currentRoom.Store(roomID)
			}
			text = RenderRoomView(p.RoomView)
		case *gamev1.ServerEvent_Message:
			text = RenderMessage(p.Message)
		case *gamev1.ServerEvent_RoomEvent:
			text = RenderRoomEvent(p.RoomEvent)
		case *gamev1.ServerEvent_PlayerList:
			text = RenderPlayerList(p.PlayerList)
		case *gamev1.ServerEvent_ExitList:
			text = RenderExitList(p.ExitList)
		case *gamev1.ServerEvent_Error:
			text = RenderError(p.Error)
		case *gamev1.ServerEvent_CombatEvent:
			text = RenderCombatEvent(p.CombatEvent)
		case *gamev1.ServerEvent_RoundStart:
			text = RenderRoundStartEvent(p.RoundStart)
		case *gamev1.ServerEvent_RoundEnd:
			text = RenderRoundEndEvent(p.RoundEnd)
		case *gamev1.ServerEvent_NpcView:
			text = RenderNpcView(p.NpcView)
		case *gamev1.ServerEvent_ConditionEvent:
			ce := p.ConditionEvent
			if ce.ConditionId == "" {
				// empty sentinel — no active conditions
				_ = conn.WriteLine(telnet.Colorize(telnet.Cyan, "No active conditions."))
				prompt := telnet.Colorf(telnet.BrightCyan, "[%s]> ", charName)
				_ = conn.WritePrompt(prompt)
				continue
			}
			text = RenderConditionEvent(ce)
		case *gamev1.ServerEvent_InventoryView:
			text = RenderInventoryView(p.InventoryView)
		case *gamev1.ServerEvent_CharacterInfo:
			text = RenderCharacterInfo(p.CharacterInfo)
		case *gamev1.ServerEvent_Disconnected:
			_ = conn.WriteLine(telnet.Colorf(telnet.Yellow, "Disconnected: %s", p.Disconnected.Reason))
			return
		}

		if text != "" {
			_ = conn.WriteLine(text)
			// Re-display prompt after each server event
			prompt := telnet.Colorf(telnet.BrightCyan, "[%s]> ", charName)
			_ = conn.WritePrompt(prompt)
		}
	}
}

// showGameHelp displays in-game help organized by category.
func (h *AuthHandler) showGameHelp(conn *telnet.Conn, registry *command.Registry, role string) {
	_ = conn.WriteLine(telnet.Colorize(telnet.BrightWhite, "Available commands:"))

	categories := []struct {
		name  string
		label string
	}{
		{command.CategoryMovement, "Movement"},
		{command.CategoryWorld, "World"},
		{command.CategoryCombat, "Combat"},
		{command.CategoryCommunication, "Communication"},
		{command.CategorySystem, "System"},
	}

	byCategory := registry.CommandsByCategory()
	for _, cat := range categories {
		cmds := byCategory[cat.name]
		if len(cmds) == 0 {
			continue
		}
		_ = conn.WriteLine(telnet.Colorf(telnet.BrightYellow, "  %s:", cat.label))
		for _, cmd := range cmds {
			aliases := ""
			if len(cmd.Aliases) > 0 {
				aliases = " (" + strings.Join(cmd.Aliases, ", ") + ")"
			}
			_ = conn.WriteLine(telnet.Colorf(telnet.Green, "    %-12s", cmd.Name) + aliases + " — " + cmd.Help)
		}
	}

	if role == postgres.RoleAdmin {
		if cmds := byCategory[command.CategoryAdmin]; len(cmds) > 0 {
			_ = conn.WriteLine(telnet.Colorf(telnet.BrightYellow, "  Admin:"))
			for _, cmd := range cmds {
				aliases := ""
				if len(cmd.Aliases) > 0 {
					aliases = " (" + strings.Join(cmd.Aliases, ", ") + ")"
				}
				_ = conn.WriteLine(telnet.Colorf(telnet.Green, "    %-12s", cmd.Name) + aliases + " — " + cmd.Help)
			}
		}
	}
}
