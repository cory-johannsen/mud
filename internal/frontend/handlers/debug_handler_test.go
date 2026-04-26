package handlers

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newDebugHandlerForTest(
	t *testing.T,
	connCount int32,
	logLines []string,
	accounts []string,
	rpc DebugRPCInvoker,
) *DebugHandler {
	t.Helper()
	logger := zaptest.NewLogger(t)
	cc := &atomic.Int32{}
	cc.Store(connCount)
	logTail := NewInMemoryLogTail(64)
	for _, ln := range logLines {
		logTail.Append(ln)
	}
	return NewDebugHandler(logger, cc, logTail, StaticWhoSource{Accounts: accounts}, rpc)
}

func TestDebugHandler_StatusReportsUptimeAndBuild(t *testing.T) {
	h := newDebugHandlerForTest(t, 3, nil, nil, nil)
	out, done, err := h.Dispatch(context.Background(), "status")
	require.NoError(t, err)
	require.False(t, done)
	require.Regexp(t, regexp.MustCompile(`uptime:\s+\S+`), out)
	require.Regexp(t, regexp.MustCompile(`build:\s+\S+`), out)
	require.Regexp(t, regexp.MustCompile(`connections:\s+3`), out)
}

func TestDebugHandler_PingEchoes(t *testing.T) {
	h := newDebugHandlerForTest(t, 0, nil, nil, nil)
	out, _, err := h.Dispatch(context.Background(), "ping")
	require.NoError(t, err)
	assert.Equal(t, "pong\r\n", out)
}

func TestDebugHandler_WhoListsConnectedAccounts(t *testing.T) {
	h := newDebugHandlerForTest(t, 0, nil, []string{"alice", "bob"}, nil)
	out, _, err := h.Dispatch(context.Background(), "who")
	require.NoError(t, err)
	assert.Contains(t, out, "alice")
	assert.Contains(t, out, "bob")
}

func TestDebugHandler_WhoEmpty(t *testing.T) {
	h := newDebugHandlerForTest(t, 0, nil, nil, nil)
	out, _, err := h.Dispatch(context.Background(), "who")
	require.NoError(t, err)
	assert.Contains(t, out, "no connected accounts")
}

func TestDebugHandler_LogTail(t *testing.T) {
	h := newDebugHandlerForTest(t, 0, []string{"level=info msg=hello"}, nil, nil)
	out, _, err := h.Dispatch(context.Background(), "log tail")
	require.NoError(t, err)
	assert.Contains(t, out, "hello")
}

func TestDebugHandler_LogTailEmpty(t *testing.T) {
	h := newDebugHandlerForTest(t, 0, nil, nil, nil)
	out, _, err := h.Dispatch(context.Background(), "log")
	require.NoError(t, err)
	assert.Contains(t, out, "log buffer empty")
}

func TestDebugHandler_LogGrep(t *testing.T) {
	h := newDebugHandlerForTest(t, 0,
		[]string{"hello world", "the quick fox", "boom shakalaka"},
		nil, nil)
	out, _, err := h.Dispatch(context.Background(), "log grep boom")
	require.NoError(t, err)
	assert.Contains(t, out, "boom")
	assert.NotContains(t, out, "hello")
}

func TestDebugHandler_LogGrepNoMatches(t *testing.T) {
	h := newDebugHandlerForTest(t, 0,
		[]string{"hello world"}, nil, nil)
	out, _, err := h.Dispatch(context.Background(), "log grep zzz")
	require.NoError(t, err)
	assert.Contains(t, out, "no matches")
}

func TestDebugHandler_LogGrepRequiresPattern(t *testing.T) {
	h := newDebugHandlerForTest(t, 0, nil, nil, nil)
	_, _, err := h.Dispatch(context.Background(), "log grep")
	require.Error(t, err)
}

type fakeRPC struct {
	resp string
	err  error
}

func (f fakeRPC) Invoke(_ context.Context, _, _ string) (string, error) {
	return f.resp, f.err
}

func TestDebugHandler_RPCInvokes(t *testing.T) {
	h := newDebugHandlerForTest(t, 0, nil, nil, fakeRPC{resp: "version=v0"})
	out, _, err := h.Dispatch(context.Background(), "rpc GetServerStatus {}")
	require.NoError(t, err)
	assert.Contains(t, out, "version")
}

func TestDebugHandler_RPCRequiresMethod(t *testing.T) {
	h := newDebugHandlerForTest(t, 0, nil, nil, nil)
	_, _, err := h.Dispatch(context.Background(), "rpc")
	require.Error(t, err)
}

func TestDebugHandler_RPCBubblesError(t *testing.T) {
	h := newDebugHandlerForTest(t, 0, nil, nil, fakeRPC{err: errors.New("boom")})
	_, _, err := h.Dispatch(context.Background(), "rpc Foo {}")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestDebugHandler_RejectsUnknownCommand(t *testing.T) {
	h := newDebugHandlerForTest(t, 0, nil, nil, nil)
	_, _, err := h.Dispatch(context.Background(), "attack goblin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown debug command")
}

func TestDebugHandler_HelpListsAllowlist(t *testing.T) {
	h := newDebugHandlerForTest(t, 0, nil, nil, nil)
	out, _, err := h.Dispatch(context.Background(), "help")
	require.NoError(t, err)
	for _, want := range []string{"status", "log", "ping", "who", "rpc", "help", "quit"} {
		assert.True(t, strings.Contains(out, want),
			"help text must mention %q", want)
	}
}

func TestDebugHandler_QuitSignalsDone(t *testing.T) {
	h := newDebugHandlerForTest(t, 0, nil, nil, nil)
	out, done, err := h.Dispatch(context.Background(), "quit")
	require.NoError(t, err)
	assert.True(t, done)
	assert.Contains(t, out, "Goodbye")
}

func TestInMemoryLogTail_RingTruncatesOldest(t *testing.T) {
	l := NewInMemoryLogTail(2)
	l.Append("a")
	l.Append("b")
	l.Append("c")
	got := l.Snapshot(0)
	assert.Equal(t, []string{"b", "c"}, got)
}

func TestInMemoryLogTail_LimitClampsToBufferSize(t *testing.T) {
	l := NewInMemoryLogTail(8)
	l.Append("only")
	got := l.Snapshot(99)
	assert.Equal(t, []string{"only"}, got)
}

func TestNoopHelpersReturnEmpty(t *testing.T) {
	assert.Nil(t, NoopLogTail{}.Snapshot(10))
	assert.Nil(t, NoopWhoSource{}.ConnectedAccounts())
	_, err := noopRPCInvoker{}.Invoke(context.Background(), "x", "{}")
	require.Error(t, err)
}
