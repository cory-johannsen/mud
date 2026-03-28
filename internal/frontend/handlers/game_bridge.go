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
	"github.com/cory-johannsen/mud/internal/gameserver"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// ErrSwitchCharacter is returned by gameBridge when the player uses the switch command.
// characterFlow checks for this sentinel to loop back to character selection.
var ErrSwitchCharacter = errors.New("switch character")

// BuildPrompt constructs the colored telnet prompt string.
//
// Precondition: maxHP > 0; name must be non-empty.
// Postcondition: Returns a non-empty string ending with "> ".
func BuildPrompt(name string, currentHP, maxHP int32, conditions []string, focusPoints, maxFocusPoints int32) string {
	// Name segment
	nameSeg := telnet.Colorf(telnet.BrightCyan, "[%s]", name)

	// HP segment — color by percentage
	if maxHP <= 0 {
		maxHP = 1
	}
	pct := float64(currentHP) / float64(maxHP)
	var hpColor string
	switch {
	case pct >= 0.75:
		hpColor = telnet.BrightGreen
	case pct >= 0.40:
		hpColor = telnet.Yellow
	default:
		hpColor = telnet.Red
	}
	hpSeg := telnet.Colorf(hpColor, "[%d/%dhp]", currentHP, maxHP)

	// Condition segments — BrightMagenta, one per active condition
	var condSegs []string
	for _, c := range conditions {
		condSegs = append(condSegs, telnet.Colorf(telnet.BrightMagenta, "[%s]", c))
	}

	parts := []string{nameSeg, hpSeg}
	parts = append(parts, condSegs...)
	prompt := strings.Join(parts, " ")

	// Focus Points segment — only shown when character has focus pool
	if maxFocusPoints > 0 {
		prompt += fmt.Sprintf(" FP: %d/%d", focusPoints, maxFocusPoints)
	}

	return prompt + "> "
}

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
				Archetype:     h.archetypeForJob(char.Class),
			},
		},
	}); err != nil {
		return fmt.Errorf("sending join request: %w", err)
	}

	// Initialize time-of-day state: default to Hour=6, Period="Dawn".
	var currentTime atomic.Value
	currentTime.Store(&gamev1.TimeOfDayEvent{Hour: 6, Period: "Dawn"})

	// Initialize GameDateTime state: default to Hour=6, Day=1, Month=1.
	var currentDT atomic.Value
	currentDT.Store(&gameserver.GameDateTime{Hour: 6, Day: 1, Month: 1})

	// Initialize HP tracking from character data.
	var currentHP atomic.Int32
	var maxHP atomic.Int32
	currentHP.Store(int32(char.CurrentHP))
	if char.MaxHP > 0 {
		maxHP.Store(int32(char.MaxHP))
	} else {
		maxHP.Store(int32(char.CurrentHP))
	}

	// Initialize Focus Point tracking; starts at zero until an HpUpdateEvent carries FP data.
	var currentFP atomic.Int32
	var maxFP atomic.Int32

	// activeConditions tracks condition ID → display name for the prompt.
	// Protected by condMu because RoomModeHandler and the event loop share it.
	var condMu sync.Mutex
	activeConditions := make(map[string]string)

	// Initialize split-screen now that the game session is starting.
	// This is deferred from acceptor so the auth/char-select flow renders
	// without a scroll region. Re-entering gameBridge (after switch) also
	// re-initializes to clear the char-select menu.
	if termW, termH := conn.Dimensions(); termW > 0 && termH > 0 {
		h.logger.Info("split-screen init", zap.Int("termW", termW), zap.Int("termH", termH))
		if err := conn.InitScreen(); err != nil {
			h.logger.Warn("split-screen init failed, falling back to scrolling mode",
				zap.Error(err))
		} else {
			conn.EnableSplitScreen()
			// Clear any prior session scrollback so the new character session
			// starts with a fresh console buffer (BUG-7).
			conn.ClearConsoleBuf()
			// Suppress client-side echo so \r\n at row H never triggers a
			// full-screen scroll.  The server echoes all characters via ReadLineSplit.
			_ = conn.SuppressEcho()
			defer conn.RestoreEcho()
		}
	}

	// lastRoomView stores the most recent *gamev1.RoomView so the resize
	// handler can re-render it at the new terminal width.  Declared before the
	// initial render so the resize handler always finds a valid value.
	var lastRoomView atomic.Value
	lastRoomView.Store((*gamev1.RoomView)(nil))

	// Receive initial room view
	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receiving initial room view: %w", err)
	}
	if rv := resp.GetRoomView(); rv != nil {
		w, _ := conn.Dimensions()
		// Seed time-of-day and date from first room view if available.
		if rv.GetPeriod() != "" {
			currentTime.Store(&gamev1.TimeOfDayEvent{Hour: rv.GetHour(), Period: rv.GetPeriod()})
			currentDT.Store(&gameserver.GameDateTime{
				Hour:  gameserver.GameHour(rv.GetHour()),
				Day:   1,
				Month: 1,
			})
		}
		dt := currentDT.Load().(*gameserver.GameDateTime)
		renderedRoom := RenderRoomView(rv, w, telnet.RoomRegionRows, *dt)
		if conn.IsSplitScreen() {
			_ = conn.WriteRoom(renderedRoom)
		} else {
			_ = conn.Write([]byte(renderedRoom))
		}
		// Seed lastRoomView so the resize handler can redraw the room region
		// without waiting for the next move/look.
		lastRoomView.Store(rv)
	}

	// Drain any pending resize signal left over from the AwaitNAWS call in the
	// acceptor.  Without this drain, forwardServerEvents fires a spurious
	// re-InitScreen immediately on startup, clearing the freshly-rendered
	// screen and pushing the initial render into scrollback.
	select {
	case <-conn.ResizeCh():
	default:
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

	// Build the input-mode handler chain now that all atomic state is initialized.
	roomHandler := NewRoomModeHandler(
		char.Name, acct.Role,
		&currentHP, &maxHP,
		&currentFP, &maxFP,
		&currentRoom, &currentTime,
		&condMu, activeConditions,
	)
	mapHandler := NewMapModeHandler()
	session := NewSessionInputState(roomHandler)

	// Initialize tab-completion channel and callback. The channel is buffered(1)
	// so forwardServerEvents can route TabCompleteResponse without blocking.
	// REQ-USE-5.
	conn.TabCompleteResponse = make(chan *gamev1.TabCompleteResponse, 1)
	conn.TabCompleter = func(prefix string) {
		msg := &gamev1.ClientMessage{
			RequestId: "req-tc-1",
			Payload: &gamev1.ClientMessage_TabComplete{
				TabComplete: &gamev1.TabCompleteRequest{Prefix: prefix},
			},
		}
		if err := stream.Send(msg); err != nil {
			h.logger.Debug("tab complete send failed", zap.Error(err))
			return
		}
		select {
		case tcResp := <-conn.TabCompleteResponse:
			renderTabCompleteResponse(conn, tcResp, session)
		case <-time.After(5 * time.Second):
			h.logger.Debug("tab complete response timed out")
		}
	}

	// Write initial prompt
	var initPromptErr error
	if conn.IsSplitScreen() {
		initPromptErr = conn.WritePromptSplit(session.CurrentPrompt())
	} else {
		initPromptErr = conn.WritePrompt(session.CurrentPrompt())
	}
	if initPromptErr != nil {
		return fmt.Errorf("writing initial prompt: %w", initPromptErr)
	}

	stopIdle := StartIdleMonitor(IdleMonitorConfig{
		LastInput:    &lastInput,
		IdleTimeout:  h.telnetCfg.IdleTimeout,
		GracePeriod:  h.telnetCfg.IdleGracePeriod,
		TickInterval: 10 * time.Second,
		OnWarning: func() {
			msg := fmt.Sprintf(
				"Warning: You have been idle for %s. You will be disconnected in %s.",
				h.telnetCfg.IdleTimeout.Round(time.Second),
				h.telnetCfg.IdleGracePeriod.Round(time.Second),
			)
			colored := telnet.Colorize(telnet.Yellow, msg)
			var err error
			if conn.IsSplitScreen() {
				err = conn.WriteConsole(colored)
			} else {
				err = conn.WriteLine(colored)
			}
			if err != nil {
				h.logger.Debug("failed to send idle warning", zap.Error(err))
			}
		},
		OnDisconnect: func() {
			msg := "Disconnecting due to inactivity."
			colored := telnet.Colorize(telnet.Red, msg)
			if conn.IsSplitScreen() {
				_ = conn.WriteConsole(colored)
			} else {
				_ = conn.WriteLine(colored)
			}
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
		h.forwardServerEvents(streamCtx, stream, conn, char.Name, &currentRoom, &currentTime, &currentDT, &currentHP, &maxHP, &currentFP, &maxFP, &lastRoomView, &condMu, activeConditions, session, mapHandler)
	}()

	// Command loop: read Telnet → parse → send gRPC
	err = h.commandLoop(streamCtx, stream, conn, char.Name, acct.Role, &lastInput, session, mapHandler, &lastRoomView)

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
// Precondition: stream must be open; charName must be non-empty; lastInput must be non-nil; session must be non-nil.
// Postcondition: Returns nil on clean quit, ctx.Err() on cancellation, or a wrapped error on failure.
func (h *AuthHandler) commandLoop(ctx context.Context, stream gamev1.GameService_SessionClient, conn *telnet.Conn, charName string, role string, lastInput *atomic.Int64, session *SessionInputState, mapHandler *MapModeHandler, lastRoomView *atomic.Value) error {
	registry := command.DefaultRegistry()
	requestID := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var line string
		var err error
		if conn.IsSplitScreen() {
			// Split-screen mode: use character-by-character reading that echoes
			// each key at row H without ever sending a newline to the terminal.
			line, err = conn.ReadLineSplit()
		} else {
			line, err = conn.ReadLine()
		}
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		lastInput.Store(time.Now().UnixNano())

		// Handle navigation sentinels.
		switch line {
		case "\x00UP":
			if conn.IsSplitScreen() {
				if cmd, ok := conn.HistoryUp(); ok {
					_ = conn.SetInputLine(session.CurrentPrompt(), cmd)
				}
			} else {
				_ = conn.WritePrompt(session.CurrentPrompt())
			}
			continue
		case "\x00DOWN":
			if conn.IsSplitScreen() {
				cmd, _ := conn.HistoryDown()
				_ = conn.SetInputLine(session.CurrentPrompt(), cmd)
			} else {
				_ = conn.WritePrompt(session.CurrentPrompt())
			}
			continue
		case "\x00SHIFT_UP":
			if conn.IsSplitScreen() {
				_ = conn.ScrollUpLine()
			}
			continue
		case "\x00SHIFT_DOWN":
			if conn.IsSplitScreen() && conn.IsScrolledBack() {
				_ = conn.ScrollDownLine()
			} else if conn.IsSplitScreen() {
				_ = conn.WritePromptSplit(session.CurrentPrompt())
			}
			continue
		case "\x00PGUP":
			if conn.IsSplitScreen() {
				_ = conn.ScrollUp()
			}
			continue
		case "\x00PGDN":
			if conn.IsSplitScreen() {
				_ = conn.ScrollDown()
			}
			continue
		}

		// Any real command while scrolled back: snap to live first.
		if conn.IsSplitScreen() {
			if snapErr := conn.SnapToLive(); snapErr != nil {
				h.logger.Debug("snap to live failed", zap.Error(snapErr))
			}
		}

		line = strings.TrimSpace(line)
		if line != "" && conn.IsSplitScreen() {
			conn.AppendHistory(line)
		}

		// Non-room mode interceptor: all modes except ModeRoom consume input via
		// their handler. ModeCombat is the exception: movement commands are blocked
		// by the handler, but non-movement commands fall through to normal gRPC
		// dispatch so the player can still attack, use items, etc.
		if session.Mode() != ModeRoom {
			if session.Mode() != ModeCombat || IsMovementCommand(line) {
				session.HandleInput(line, conn, stream, &requestID)
				continue
			}
		}

		if line == "" {
			if conn.IsSplitScreen() {
				_ = conn.WritePromptSplit(session.CurrentPrompt())
			} else {
				_ = conn.WritePrompt(session.CurrentPrompt())
			}
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

		travelResolver := func(zoneName string) (string, string) {
			_, _, resp := mapHandler.Snapshot()
			if resp == nil {
				return "", "Open the world map first with 'map'."
			}
			lower := strings.ToLower(zoneName)
			var matches []*gamev1.WorldZoneTile
			for _, t := range resp.GetWorldTiles() {
				if strings.HasPrefix(strings.ToLower(t.ZoneName), lower) {
					matches = append(matches, t)
				}
			}
			if len(matches) == 0 {
				return "", "No such zone."
			}
			// Tiebreak: lexicographic zone ID order.
			sortWorldTiles(matches)
			return matches[0].ZoneId, ""
		}

		bctx := &bridgeContext{
			reqID:          reqID,
			cmd:            cmd,
			parsed:         parsed,
			conn:           conn,
			charName:       charName,
			role:           role,
			stream:         stream,
			promptFn:       session.CurrentPrompt,
			travelResolver: travelResolver,
			roomViewFn: func() *gamev1.RoomView {
				if rv, ok := lastRoomView.Load().(*gamev1.RoomView); ok {
					return rv
				}
				return nil
			},
			helpFn: func() {
				h.showGameHelp(conn, registry, role)
				if conn.IsSplitScreen() {
					_ = conn.WritePromptSplit(session.CurrentPrompt())
				} else {
					_ = conn.WritePrompt(session.CurrentPrompt())
				}
			},
		}

		handlerFn, ok := bridgeHandlerMap[cmd.Handler]
		if !ok {
			msg := telnet.Colorf(telnet.Dim, "You don't know how to '%s'.", parsed.Command)
			if conn.IsSplitScreen() {
				_ = conn.WriteConsole(msg)
				_ = conn.WritePromptSplit(session.CurrentPrompt())
			} else {
				_ = conn.WriteLine(msg)
				_ = conn.WritePrompt(session.CurrentPrompt())
			}
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
		if result.enterMapMode {
			mapHandler.SetView(result.mapView)
			session.SetMode(conn, mapHandler)
		}
		if result.done {
			if result.consoleMsg != "" {
				if conn.IsSplitScreen() {
					_ = conn.WriteConsole(result.consoleMsg)
				} else {
					_ = conn.WriteLine(result.consoleMsg)
				}
			}
			if conn.IsSplitScreen() {
				_ = conn.WritePromptSplit(session.CurrentPrompt())
			} else {
				_ = conn.WritePrompt(session.CurrentPrompt())
			}
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

// renderTabCompleteResponse displays tab-completion candidates on the console.
//
// Precondition: conn must be non-nil; resp must be non-nil; session must be non-nil.
// Postcondition: Writes a human-readable completion list or "No completions found." to the console.
// REQ-USE-10, REQ-USE-11.
func renderTabCompleteResponse(conn *telnet.Conn, resp *gamev1.TabCompleteResponse, session *SessionInputState) {
	completions := resp.GetCompletions()
	var msg string
	switch {
	case len(completions) == 0:
		msg = telnet.Colorize(telnet.Dim, "No completions found.")
	case len(completions) == 1:
		msg = completions[0]
	case len(completions) <= 10:
		msg = strings.Join(completions, "  ")
	default:
		first10 := strings.Join(completions[:10], "  ")
		remaining := len(completions) - 10
		msg = fmt.Sprintf("%s  ... (%d more)", first10, remaining)
	}
	if conn.IsSplitScreen() {
		_ = conn.WriteConsole(msg)
		_ = conn.WritePromptSplit(session.CurrentPrompt())
	} else {
		_ = conn.WriteLine(msg)
		_ = conn.WritePrompt(session.CurrentPrompt())
	}
}

// forwardServerEvents reads ServerEvents from the gRPC stream and writes
// rendered text to the Telnet connection.
//
// Precondition: stream must be open; charName must be non-empty; currentRoom, currentTime, currentDT, currentHP, maxHP must be non-nil; session must be non-nil.
// Postcondition: Returns when ctx is done, stream closes, or a disconnect event is received.
// Side-effect: currentRoom is updated to the latest RoomView.RoomId whenever a RoomView event is received.
// Side-effect: currentTime and currentDT are updated from TimeOfDayEvent or RoomView events.
// Side-effect: currentHP, maxHP, currentFP, and maxFP are updated from CharacterInfo and HpUpdateEvent events.
func (h *AuthHandler) forwardServerEvents(ctx context.Context, stream gamev1.GameService_SessionClient, conn *telnet.Conn, charName string, currentRoom *atomic.Value, currentTime *atomic.Value, currentDT *atomic.Value, currentHP *atomic.Int32, maxHP *atomic.Int32, currentFP *atomic.Int32, maxFP *atomic.Int32, lastRoomView *atomic.Value, condMu *sync.Mutex, activeConditions map[string]string, session *SessionInputState, mapHandler *MapModeHandler) {
	// Pump stream.Recv() into a channel so it can participate in a proper
	// select alongside resize events and the prompt-refresh ticker.
	type recvResult struct {
		resp *gamev1.ServerEvent
		err  error
	}
	combatHandler := NewCombatModeHandler(charName, func() {})
	var combatEndTimer *time.Timer

	recvCh := make(chan recvResult, 4)
	go func() {
		for {
			resp, err := stream.Recv()
			recvCh <- recvResult{resp, err}
			if err != nil {
				return
			}
		}
	}()

	// Refresh the prompt on every clock tick (game_tick_duration = 1 min).
	// This ensures the time in the prompt advances even when no other server
	// event arrives.
	promptTicker := time.NewTicker(30 * time.Second)
	defer promptTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-conn.ResizeCh():
			if conn.IsSplitScreen() {
				rw, rh := conn.Dimensions()
				h.logger.Info("split-screen resize", zap.Int("termW", rw), zap.Int("termH", rh))
				if err := conn.InitScreen(); err != nil {
					h.logger.Warn("split-screen resize init failed", zap.Error(err))
					continue
				}
				if session.Mode() == ModeMap {
					mapView, _, mapResp := mapHandler.Snapshot()
					if mapResp != nil {
						_ = conn.WriteConsole(renderMapConsole(mapResp, mapView, rw))
					}
					_ = conn.WritePromptSplit(session.CurrentPrompt())
					continue
				}
				if session.Mode() == ModeCombat {
					snap := combatHandler.SnapshotForRender()
					_ = conn.WriteRoom(RenderCombatScreen(snap, rw))
					_ = conn.WritePromptSplit(session.CurrentPrompt())
					continue
				}
				if rv, ok := lastRoomView.Load().(*gamev1.RoomView); ok && rv != nil {
					dt := currentDT.Load().(*gameserver.GameDateTime)
					_ = conn.WriteRoom(RenderRoomView(rv, rw, telnet.RoomRegionRows, *dt))
				}
				_ = conn.WritePromptSplit(session.CurrentPrompt())
			}
			continue
		case <-promptTicker.C:
			if conn.IsSplitScreen() {
				_ = conn.WritePromptSplit(session.CurrentPrompt())
			}
			continue
		case rr := <-recvCh:
			if rr.err != nil {
				if rr.err != io.EOF && ctx.Err() == nil {
					h.logger.Debug("stream recv error in forwarder", zap.Error(rr.err))
				}
				return
			}
			resp := rr.resp

			var text string
			switch p := resp.Payload.(type) {
		case *gamev1.ServerEvent_TimeOfDay:
			currentTime.Store(p.TimeOfDay)
			dt := &gameserver.GameDateTime{
				Hour:  gameserver.GameHour(p.TimeOfDay.GetHour()),
				Day:   int(p.TimeOfDay.GetDay()),
				Month: int(p.TimeOfDay.GetMonth()),
			}
			currentDT.Store(dt)
			if conn.IsSplitScreen() {
				_ = conn.WritePromptSplit(session.CurrentPrompt())
			} else {
				_ = conn.WritePrompt(session.CurrentPrompt())
			}
			continue
		case *gamev1.ServerEvent_RoomView:
			// Exit map mode only when the player has actually changed rooms (travel/movement).
			// Ambient refreshes of the current room (same room ID) must not disrupt map mode.
			// Never exit combat mode due to a room view — combat manages its own region.
			if session.Mode() != ModeRoom && session.Mode() != ModeCombat {
				incomingID := p.RoomView.GetRoomId()
				existingID, _ := currentRoom.Load().(string)
				if incomingID == "" || incomingID != existingID {
					session.SetMode(conn, session.Room())
				}
			}
			if roomID := p.RoomView.GetRoomId(); roomID != "" {
				currentRoom.Store(roomID)
			}
			if p.RoomView.GetPeriod() != "" {
				currentTime.Store(&gamev1.TimeOfDayEvent{Hour: p.RoomView.GetHour(), Period: p.RoomView.GetPeriod()})
				if existing, ok := currentDT.Load().(*gameserver.GameDateTime); ok && existing != nil {
					currentDT.Store(&gameserver.GameDateTime{
						Hour:  gameserver.GameHour(p.RoomView.GetHour()),
						Day:   existing.Day,
						Month: existing.Month,
					})
				}
			}
			lastRoomView.Store(p.RoomView)
			// Suppress room region render during combat — the combat screen owns that region.
			if session.Mode() == ModeCombat {
				continue
			}
			w, _ := conn.Dimensions()
			rvDT := currentDT.Load().(*gameserver.GameDateTime)
			text = RenderRoomView(p.RoomView, w, telnet.RoomRegionRows, *rvDT)
		case *gamev1.ServerEvent_Message:
			period := ""
			if tod, ok := currentTime.Load().(*gamev1.TimeOfDayEvent); ok && tod != nil {
				period = tod.GetPeriod()
			}
			text = RenderMessage(p.Message, period)
		case *gamev1.ServerEvent_RoomEvent:
			text = RenderRoomEvent(p.RoomEvent)
		case *gamev1.ServerEvent_PlayerList:
			text = RenderPlayerList(p.PlayerList)
		case *gamev1.ServerEvent_ExitList:
			text = RenderExitList(p.ExitList)
		case *gamev1.ServerEvent_Error:
			text = RenderError(p.Error)
		case *gamev1.ServerEvent_CombatEvent:
			ce := p.CombatEvent
			// Handle position events: update combatant position and re-render the battlefield.
			if ce.GetType() == gamev1.CombatEventType_COMBAT_EVENT_TYPE_POSITION {
				combatHandler.UpdatePosition(ce.GetAttacker(), int(ce.GetAttackerPosition()))
				if session.Mode() == ModeCombat {
					cw, _ := conn.Dimensions()
					snap := combatHandler.SnapshotForRender()
					if conn.IsSplitScreen() {
						_ = conn.WriteRoom(RenderCombatScreen(snap, cw))
					}
				}
				continue
			}
			// Update current HP when this player is the attack target.
			if ce.GetTarget() == charName && ce.GetTargetHp() != 0 {
				currentHP.Store(ce.GetTargetHp())
			}
			combatHandler.UpdateCombatEvent(
				ce.GetAttacker(), ce.GetTarget(),
				int(ce.GetDamage()), int(ce.GetTargetHp()), int(ce.GetTargetMaxHp()),
				ce.GetNarrative(), int32(ce.GetType()),
			)
			if ce.GetType() == gamev1.CombatEventType_COMBAT_EVENT_TYPE_END {
				combatHandler.SetSummary("Combat complete.")
				cw, _ := conn.Dimensions()
				summary := RenderCombatSummary("Combat complete.", cw)
				if conn.IsSplitScreen() {
					_ = conn.WriteRoom(summary)
					_ = conn.WritePromptSplit(combatHandler.Prompt())
				} else {
					_ = conn.WriteLine(summary)
					_ = conn.WritePrompt(combatHandler.Prompt())
				}
				combatEndTimer = time.AfterFunc(3*time.Second, func() {
					session.SetMode(conn, session.Room())
					// Re-render the room view now that the combat screen is gone.
					rv, _ := lastRoomView.Load().(*gamev1.RoomView)
					if rv != nil {
						rw, _ := conn.Dimensions()
						dt := currentDT.Load().(*gameserver.GameDateTime)
						if dt == nil {
							dt = &gameserver.GameDateTime{}
						}
						roomScreen := RenderRoomView(rv, rw, telnet.RoomRegionRows, *dt)
						if conn.IsSplitScreen() {
							_ = conn.WriteRoom(roomScreen)
							_ = conn.WritePromptSplit(session.CurrentPrompt())
						} else {
							_ = conn.WriteLine(roomScreen)
							_ = conn.WritePrompt(session.CurrentPrompt())
						}
					}
				})
				continue
			}
			// Re-render combat screen for non-END events.
			if session.Mode() == ModeCombat {
				cw, _ := conn.Dimensions()
				snap := combatHandler.SnapshotForRender()
				if conn.IsSplitScreen() {
					_ = conn.WriteRoom(RenderCombatScreen(snap, cw))
				}
			}
			text = RenderCombatEvent(ce)
		case *gamev1.ServerEvent_RoundStart:
			rs := p.RoundStart
			// Cancel any pending combat-end timer to prevent it from
			// yanking the player out of a new combat that started quickly.
			if combatEndTimer != nil {
				combatEndTimer.Stop()
				combatEndTimer = nil
			}
			// Transition to combat mode on first round.
			if session.Mode() != ModeCombat {
				combatHandler.Reset()
				session.SetMode(conn, combatHandler)
			}
			turnOrder := make([]string, len(rs.GetTurnOrder()))
			copy(turnOrder, rs.GetTurnOrder())
			combatHandler.UpdateRoundStart(int(rs.GetRound()), int(rs.GetActionsPerTurn()), turnOrder)
			// Seed player HP from stored values so the HP bar shows immediately.
			combatHandler.UpdatePlayerHP(int(currentHP.Load()), int(maxHP.Load()))
			// Render combat screen in room region.
			cw, _ := conn.Dimensions()
			snap := combatHandler.SnapshotForRender()
			if conn.IsSplitScreen() {
				_ = conn.WriteRoom(RenderCombatScreen(snap, cw))
				_ = conn.WritePromptSplit(combatHandler.Prompt())
			} else {
				_ = conn.WriteLine(RenderCombatScreen(snap, cw))
				_ = conn.WritePrompt(combatHandler.Prompt())
			}
			// Also write the text-mode round start to console for the combat log.
			text = RenderRoundStartEvent(rs)
		case *gamev1.ServerEvent_RoundEnd:
			text = RenderRoundEndEvent(p.RoundEnd)
		case *gamev1.ServerEvent_NpcView:
			text = RenderNpcView(p.NpcView)
		case *gamev1.ServerEvent_ConditionEvent:
			ce := p.ConditionEvent
			if ce.ConditionId == "" {
				if conn.IsSplitScreen() {
					_ = conn.WriteConsole(telnet.Colorize(telnet.Cyan, "No active conditions."))
					_ = conn.WritePromptSplit(session.CurrentPrompt())
				} else {
					_ = conn.WriteLine(telnet.Colorize(telnet.Cyan, "No active conditions."))
					_ = conn.WritePrompt(session.CurrentPrompt())
				}
				continue
			}
			condMu.Lock()
			if ce.GetApplied() {
				activeConditions[ce.GetConditionId()] = ce.GetConditionName()
			} else {
				delete(activeConditions, ce.GetConditionId())
			}
			if session.Mode() == ModeCombat {
				condNames := make([]string, 0, len(activeConditions))
				for _, name := range activeConditions {
					condNames = append(condNames, name)
				}
				condMu.Unlock()
				combatHandler.UpdateConditions(condNames)
				cw, _ := conn.Dimensions()
				snap := combatHandler.SnapshotForRender()
				if conn.IsSplitScreen() {
					_ = conn.WriteRoom(RenderCombatScreen(snap, cw))
				}
			} else {
				condMu.Unlock()
			}
			text = RenderConditionEvent(ce)
		case *gamev1.ServerEvent_InventoryView:
			text = RenderInventoryView(p.InventoryView)
		case *gamev1.ServerEvent_CharacterInfo:
			ci := p.CharacterInfo
			if ci.GetMaxHp() > 0 {
				maxHP.Store(ci.GetMaxHp())
			}
			currentHP.Store(ci.GetCurrentHp())
			text = RenderCharacterInfo(ci)
		case *gamev1.ServerEvent_CharacterSheet:
			cw, _ := conn.Dimensions()
			text = RenderCharacterSheet(p.CharacterSheet, cw)
		case *gamev1.ServerEvent_Map:
			mw, _ := conn.Dimensions()
			if session.Mode() == ModeMap {
				mapHandler.SetLastResponse(p.Map)
				mapView, _, _ := mapHandler.Snapshot()
				var rendered string
				if mapView == "world" {
					rendered = RenderWorldMap(p.Map, mw)
				} else {
					rendered = RenderMap(p.Map, mw)
				}
				if conn.IsSplitScreen() {
					_ = conn.WriteConsole(rendered)
					_ = conn.WritePromptSplit(session.CurrentPrompt())
				} else {
					_ = conn.WriteLine(rendered)
					_ = conn.WritePrompt(session.CurrentPrompt())
				}
				continue
			}
			text = RenderMap(p.Map, mw)
		case *gamev1.ServerEvent_SkillsResponse:
			text = RenderSkillsResponse(p.SkillsResponse)
		case *gamev1.ServerEvent_FeatsResponse:
			text = RenderFeatsResponse(p.FeatsResponse)
		case *gamev1.ServerEvent_ClassFeaturesResponse:
			text = RenderClassFeaturesResponse(p.ClassFeaturesResponse)
		case *gamev1.ServerEvent_ProficienciesResponse:
			text = RenderProficienciesResponse(p.ProficienciesResponse)
		case *gamev1.ServerEvent_InteractResponse:
			text = RenderInteractResponse(p.InteractResponse)
		case *gamev1.ServerEvent_UseResponse:
			text = RenderUseResponse(p.UseResponse)
		case *gamev1.ServerEvent_HpUpdate:
			hpu := p.HpUpdate
			currentHP.Store(hpu.GetCurrentHp())
			if hpu.GetMaxHp() > 0 {
				maxHP.Store(hpu.GetMaxHp())
			}
			currentFP.Store(hpu.GetFocusPoints())
			if hpu.GetMaxFocusPoints() > 0 {
				maxFP.Store(hpu.GetMaxFocusPoints())
			}
			if session.Mode() == ModeCombat {
				combatHandler.UpdatePlayerHP(int(hpu.GetCurrentHp()), int(hpu.GetMaxHp()))
				cw, _ := conn.Dimensions()
				snap := combatHandler.SnapshotForRender()
				if conn.IsSplitScreen() {
					_ = conn.WriteRoom(RenderCombatScreen(snap, cw))
				}
			}
			if conn.IsSplitScreen() {
				_ = conn.WritePromptSplit(session.CurrentPrompt())
			} else {
				_ = conn.WritePrompt(session.CurrentPrompt())
			}
			continue
		case *gamev1.ServerEvent_Disconnected:
			dcMsg := telnet.Colorf(telnet.Yellow, "Disconnected: %s", p.Disconnected.Reason)
			if conn.IsSplitScreen() {
				_ = conn.WriteConsole(dcMsg)
			} else {
				_ = conn.WriteLine(dcMsg)
			}
			return
		case *gamev1.ServerEvent_TabComplete:
			// Route to the dedicated channel; the TabCompleter callback reads it.
			// Non-blocking send: drop if no one is waiting (e.g. TabCompleter timed out).
			// REQ-USE-5.
			select {
			case conn.TabCompleteResponse <- p.TabComplete:
			default:
			}
			continue
		}

			if text != "" {
				if conn.IsSplitScreen() {
					switch resp.Payload.(type) {
					case *gamev1.ServerEvent_RoomView:
						_ = conn.WriteRoom(text)
					default:
						_ = conn.WriteConsole(text)
					}
					_ = conn.WritePromptSplit(session.CurrentPrompt())
				} else {
					_ = conn.WriteLine(text)
					// Re-display prompt after each server event
					_ = conn.WritePrompt(session.CurrentPrompt())
				}
			}
		} // end select
	} // end for
}

// archetypeForJob returns the Archetype string for the job with the given ID.
// If no matching job is found, it returns an empty string.
//
// Precondition: jobID must be non-empty.
// Postcondition: Returns the archetype string or "" if the job is not registered.
func (h *AuthHandler) archetypeForJob(jobID string) string {
	for _, j := range h.jobs {
		if j.ID == jobID {
			return j.Archetype
		}
	}
	return ""
}

// showGameHelp displays in-game help organized by category.
// In split-screen mode, the entire help block is sent as a single WriteConsole call
// to avoid full-terminal scrolls from individual WriteLine calls at row H.
func (h *AuthHandler) showGameHelp(conn *telnet.Conn, registry *command.Registry, role string) {
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

	if conn.IsSplitScreen() {
		var sb strings.Builder
		sb.WriteString(telnet.Colorize(telnet.BrightWhite, "Available commands:"))
		for _, cat := range categories {
			cmds := byCategory[cat.name]
			if len(cmds) == 0 {
				continue
			}
			sb.WriteString("\r\n")
			sb.WriteString(telnet.Colorf(telnet.BrightYellow, "  %s:", cat.label))
			for _, cmd := range cmds {
				aliases := ""
				if len(cmd.Aliases) > 0 {
					aliases = " (" + strings.Join(cmd.Aliases, ", ") + ")"
				}
				sb.WriteString("\r\n")
				sb.WriteString(telnet.Colorf(telnet.Green, "    %-12s", cmd.Name) + aliases + " — " + cmd.Help)
			}
		}
		if role == postgres.RoleAdmin {
			if cmds := byCategory[command.CategoryAdmin]; len(cmds) > 0 {
				sb.WriteString("\r\n")
				sb.WriteString(telnet.Colorf(telnet.BrightYellow, "  Admin:"))
				for _, cmd := range cmds {
					aliases := ""
					if len(cmd.Aliases) > 0 {
						aliases = " (" + strings.Join(cmd.Aliases, ", ") + ")"
					}
					sb.WriteString("\r\n")
					sb.WriteString(telnet.Colorf(telnet.Green, "    %-12s", cmd.Name) + aliases + " — " + cmd.Help)
				}
			}
		}
		if role == postgres.RoleEditor || role == postgres.RoleAdmin {
			if cmds := byCategory[command.CategoryEditor]; len(cmds) > 0 {
				sb.WriteString("\r\n")
				sb.WriteString(telnet.Colorf(telnet.BrightYellow, "  Editor:"))
				for _, cmd := range cmds {
					aliases := ""
					if len(cmd.Aliases) > 0 {
						aliases = " (" + strings.Join(cmd.Aliases, ", ") + ")"
					}
					sb.WriteString("\r\n")
					sb.WriteString(telnet.Colorf(telnet.Green, "    %-12s", cmd.Name) + aliases + " — " + cmd.Help)
				}
			}
		}
		_ = conn.WriteConsole(sb.String())
		return
	}

	_ = conn.WriteLine(telnet.Colorize(telnet.BrightWhite, "Available commands:"))
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
	if role == postgres.RoleEditor || role == postgres.RoleAdmin {
		if cmds := byCategory[command.CategoryEditor]; len(cmds) > 0 {
			_ = conn.WriteLine(telnet.Colorf(telnet.BrightYellow, "  Editor:"))
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
