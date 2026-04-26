package handlers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/version"
)

// DebugHandler implements telnet.SessionHandler for the loopback-only
// headless debug surface (REQ-TD-3 / REQ-TD-4 — telnet-deprecation #325).
//
// Allowed commands form a small operator-friendly allowlist:
//
//	status            — uptime / version / build / connection count
//	log               — most recent buffered log lines (subcommands: tail | grep <pattern>)
//	ping              — round-trip echo (responds "pong")
//	who               — list connected accounts (loopback-only; safe to expose)
//	rpc <method> <json>
//	                  — dispatch a debug RPC by name to the registered RPC dispatcher;
//	                    returns a string response
//	help              — print the allowlist
//	quit / exit       — close the session
//
// Game commands (look, attack, move, ...) are NOT available on this surface.
// They remain reachable only via the legacy player flow and only when
// telnet.allow_game_commands is set to true.
type DebugHandler struct {
	logger     *zap.Logger
	startedAt  time.Time
	connCount  *atomic.Int32
	logTail    LogTail
	whoSource  WhoSource
	rpcInvoker DebugRPCInvoker
}

// LogTail abstracts a recent-log accessor used by `log tail` and `log grep`.
// Implementations MUST be goroutine-safe; the debug shell reads from
// goroutines spawned per-connection.
type LogTail interface {
	// Snapshot returns up to limit recent log lines, oldest-first.
	Snapshot(limit int) []string
}

// WhoSource lists currently-connected accounts.
type WhoSource interface {
	ConnectedAccounts() []string
}

// DebugRPCInvoker dispatches the `rpc <method> <json>` form to a backend.
// The implementation is responsible for marshalling JSON args and returning
// a printable response.
type DebugRPCInvoker interface {
	Invoke(ctx context.Context, method, jsonArgs string) (string, error)
}

// NoopLogTail returns no lines. Used when no log buffer is wired.
type NoopLogTail struct{}

// Snapshot returns nil (no-op).
func (NoopLogTail) Snapshot(_ int) []string { return nil }

// NoopWhoSource returns no accounts. Used when no connection registry is wired.
type NoopWhoSource struct{}

// ConnectedAccounts returns an empty slice (no-op).
func (NoopWhoSource) ConnectedAccounts() []string { return nil }

// noopRPCInvoker rejects all RPC invocations.
type noopRPCInvoker struct{}

func (noopRPCInvoker) Invoke(_ context.Context, _, _ string) (string, error) {
	return "", fmt.Errorf("debug rpc not configured")
}

// InMemoryLogTail is a goroutine-safe ring buffer of recent log lines for
// debug-shell consumption. Use it as the LogTail implementation when no
// external log store is available.
type InMemoryLogTail struct {
	mu       sync.Mutex
	capacity int
	lines    []string
}

// NewInMemoryLogTail constructs an InMemoryLogTail with the given ring
// capacity. capacity <= 0 is clamped to 1.
func NewInMemoryLogTail(capacity int) *InMemoryLogTail {
	if capacity <= 0 {
		capacity = 1
	}
	return &InMemoryLogTail{capacity: capacity}
}

// Append records a line. Older lines are evicted past capacity.
func (l *InMemoryLogTail) Append(line string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, line)
	if len(l.lines) > l.capacity {
		l.lines = l.lines[len(l.lines)-l.capacity:]
	}
}

// Snapshot returns up to limit recent lines (oldest-first). limit <= 0
// returns the full ring.
func (l *InMemoryLogTail) Snapshot(limit int) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if limit <= 0 || limit > len(l.lines) {
		limit = len(l.lines)
	}
	out := make([]string, limit)
	copy(out, l.lines[len(l.lines)-limit:])
	return out
}

// StaticWhoSource is a fixed-account WhoSource useful for tests.
type StaticWhoSource struct {
	Accounts []string
}

// ConnectedAccounts returns the configured account list.
func (s StaticWhoSource) ConnectedAccounts() []string { return s.Accounts }

// NewDebugHandler constructs a DebugHandler. logger is required; the other
// dependencies fall back to no-op implementations when nil.
func NewDebugHandler(
	logger *zap.Logger,
	connCount *atomic.Int32,
	logTail LogTail,
	whoSource WhoSource,
	rpcInvoker DebugRPCInvoker,
) *DebugHandler {
	if connCount == nil {
		connCount = &atomic.Int32{}
	}
	if logTail == nil {
		logTail = NoopLogTail{}
	}
	if whoSource == nil {
		whoSource = NoopWhoSource{}
	}
	if rpcInvoker == nil {
		rpcInvoker = noopRPCInvoker{}
	}
	return &DebugHandler{
		logger:     logger,
		startedAt:  time.Now(),
		connCount:  connCount,
		logTail:    logTail,
		whoSource:  whoSource,
		rpcInvoker: rpcInvoker,
	}
}

// HandleSession implements telnet.SessionHandler.
//
// Precondition: conn must be non-nil.
// Postcondition: returns nil on clean quit, or a non-nil error if the
// connection failed mid-session.
func (h *DebugHandler) HandleSession(ctx context.Context, conn *telnet.Conn) error {
	h.connCount.Add(1)
	defer h.connCount.Add(-1)

	if err := conn.WriteLine("Gunchete debug shell. Type `help` for commands."); err != nil {
		return fmt.Errorf("debug greeting: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := conn.WritePrompt("debug> "); err != nil {
			return fmt.Errorf("debug prompt: %w", err)
		}
		line, err := conn.ReadLine()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("debug read: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		out, done, err := h.dispatch(ctx, line)
		if err != nil {
			_ = conn.WriteLine("error: " + err.Error())
			continue
		}
		if out != "" {
			if err := conn.Write([]byte(out)); err != nil {
				return fmt.Errorf("debug write: %w", err)
			}
		}
		if done {
			return nil
		}
	}
}

// Dispatch is exposed for unit tests so they can drive the command parser
// without standing up a Conn.
//
// Returns: (output, done, err). done==true requests session termination.
func (h *DebugHandler) Dispatch(ctx context.Context, line string) (string, bool, error) {
	return h.dispatch(ctx, strings.TrimSpace(line))
}

func (h *DebugHandler) dispatch(ctx context.Context, line string) (string, bool, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", false, nil
	}
	switch strings.ToLower(fields[0]) {
	case "status":
		return h.handleStatus(), false, nil
	case "ping":
		return "pong\r\n", false, nil
	case "who":
		return h.handleWho(), false, nil
	case "log":
		out, err := h.handleLog(fields[1:])
		return out, false, err
	case "rpc":
		out, err := h.handleRPC(ctx, fields[1:])
		return out, false, err
	case "help":
		return h.handleHelp(), false, nil
	case "quit", "exit":
		return "Goodbye!\r\n", true, nil
	default:
		return "", false, fmt.Errorf("unknown debug command %q (try `help`)", fields[0])
	}
}

func (h *DebugHandler) handleStatus() string {
	uptime := time.Since(h.startedAt).Truncate(time.Second)
	return fmt.Sprintf(
		"uptime: %s\r\nversion: %s\r\nbuild: %s\r\nconnections: %d\r\n",
		uptime, version.Version, version.Version, h.connCount.Load(),
	)
}

func (h *DebugHandler) handleWho() string {
	accts := h.whoSource.ConnectedAccounts()
	if len(accts) == 0 {
		return "(no connected accounts)\r\n"
	}
	var b strings.Builder
	for _, a := range accts {
		b.WriteString(a)
		b.WriteString("\r\n")
	}
	return b.String()
}

func (h *DebugHandler) handleLog(args []string) (string, error) {
	sub := "tail"
	if len(args) > 0 {
		sub = strings.ToLower(args[0])
	}
	switch sub {
	case "tail":
		lines := h.logTail.Snapshot(20)
		if len(lines) == 0 {
			return "(log buffer empty)\r\n", nil
		}
		return strings.Join(lines, "\r\n") + "\r\n", nil
	case "grep":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: log grep <pattern>")
		}
		pat := strings.Join(args[1:], " ")
		lines := h.logTail.Snapshot(0)
		var matched []string
		for _, ln := range lines {
			if strings.Contains(ln, pat) {
				matched = append(matched, ln)
			}
		}
		if len(matched) == 0 {
			return "(no matches)\r\n", nil
		}
		return strings.Join(matched, "\r\n") + "\r\n", nil
	default:
		return "", fmt.Errorf("unknown log subcommand %q (use `tail` or `grep`)", sub)
	}
}

func (h *DebugHandler) handleRPC(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: rpc <method> [json-args]")
	}
	method := args[0]
	jsonArgs := ""
	if len(args) > 1 {
		jsonArgs = strings.Join(args[1:], " ")
	}
	out, err := h.rpcInvoker.Invoke(ctx, method, jsonArgs)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\r\n"
	}
	return out, nil
}

func (h *DebugHandler) handleHelp() string {
	return strings.Join([]string{
		"Available debug commands (telnet-deprecation #325):",
		"  status          - server uptime / version / build / connection count",
		"  ping            - round-trip echo",
		"  who             - list connected accounts",
		"  log [tail|grep <pat>]  - recent log entries (default: tail)",
		"  rpc <method> [json]    - dispatch a debug RPC by name",
		"  help            - this help",
		"  quit | exit     - close the session",
		"",
	}, "\r\n")
}

// readDebugLine reads a single \n-terminated line from r. Used by tests that
// drive a Pipe-backed Conn without going through the full telnet IAC loop.
func readDebugLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
