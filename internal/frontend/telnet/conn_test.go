package telnet

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestFilterIAC_NoIAC(t *testing.T) {
	input := []byte("hello world")
	result := FilterIAC(input)
	assert.Equal(t, input, result)
}

func TestFilterIAC_WillCommand(t *testing.T) {
	input := []byte{IAC, WILL, OptEcho, 'h', 'i'}
	result := FilterIAC(input)
	assert.Equal(t, []byte("hi"), result)
}

func TestFilterIAC_WontCommand(t *testing.T) {
	input := []byte{IAC, WONT, OptSuppressGoAhead, 'o', 'k'}
	result := FilterIAC(input)
	assert.Equal(t, []byte("ok"), result)
}

func TestFilterIAC_DoCommand(t *testing.T) {
	input := []byte{'a', IAC, DO, OptLinemode, 'b'}
	result := FilterIAC(input)
	assert.Equal(t, []byte("ab"), result)
}

func TestFilterIAC_DontCommand(t *testing.T) {
	input := []byte{IAC, DONT, OptEcho}
	result := FilterIAC(input)
	assert.Empty(t, result)
}

func TestFilterIAC_SubNegotiation(t *testing.T) {
	input := []byte{IAC, SB, 24, 0, 'x', 't', 'e', 'r', 'm', IAC, SE, 'z'}
	result := FilterIAC(input)
	assert.Equal(t, []byte("z"), result)
}

func TestFilterIAC_EscapedIAC(t *testing.T) {
	// IAC IAC is an escaped 0xFF; in text-stripping mode both bytes are discarded
	// to prevent the emitted 0xFF from forming a false IAC+cmd sequence.
	input := []byte{'a', IAC, IAC, 'b'}
	result := FilterIAC(input)
	assert.Equal(t, []byte{'a', 'b'}, result)
}

func TestFilterIAC_NOP(t *testing.T) {
	input := []byte{'x', IAC, NOP, 'y'}
	result := FilterIAC(input)
	assert.Equal(t, []byte("xy"), result)
}

func TestFilterIAC_MultipleCommands(t *testing.T) {
	input := []byte{
		IAC, WILL, OptSuppressGoAhead,
		IAC, WILL, OptEcho,
		'h', 'e', 'l', 'l', 'o',
	}
	result := FilterIAC(input)
	assert.Equal(t, []byte("hello"), result)
}

// --- Conn method tests using net.Pipe ---

func newTestConn(t *testing.T) (*Conn, net.Conn) {
	t.Helper()
	client, server := net.Pipe()
	conn := NewConn(server, 2*time.Second, 2*time.Second)
	t.Cleanup(func() {
		client.Close()
		conn.Close()
	})
	return conn, client
}

func TestConn_WriteLine(t *testing.T) {
	conn, client := newTestConn(t)

	go func() {
		err := conn.WriteLine("hello world")
		assert.NoError(t, err)
	}()

	buf := make([]byte, 256)
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := client.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "hello world\r\n", string(buf[:n]))
}

func TestConn_Write(t *testing.T) {
	conn, client := newTestConn(t)

	data := []byte{0x01, 0x02, 0x03}
	go func() {
		err := conn.Write(data)
		assert.NoError(t, err)
	}()

	buf := make([]byte, 256)
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := client.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, data, buf[:n])
}

func TestConn_WritePrompt(t *testing.T) {
	conn, client := newTestConn(t)

	go func() {
		err := conn.WritePrompt("> ")
		assert.NoError(t, err)
	}()

	buf := make([]byte, 256)
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := client.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "> ", string(buf[:n]))
}

func TestConn_RemoteAddr(t *testing.T) {
	conn, _ := newTestConn(t)
	addr := conn.RemoteAddr()
	assert.NotNil(t, addr)
}

func TestConn_ReadLine_Simple(t *testing.T) {
	conn, client := newTestConn(t)

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, _ = client.Write([]byte("hello\r\n"))
	}()

	line, err := conn.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "hello", line)
}

func TestConn_ReadLine_CROnly(t *testing.T) {
	conn, client := newTestConn(t)

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, _ = client.Write([]byte("test\r"))
		client.Close() // Close so Peek returns EOF immediately instead of blocking
	}()

	line, err := conn.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "test", line)
}

func TestConn_ReadLine_LFOnly(t *testing.T) {
	conn, client := newTestConn(t)

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, _ = client.Write([]byte("test\n"))
	}()

	line, err := conn.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "test", line)
}

func TestConn_ReadLine_FiltersControlChars(t *testing.T) {
	conn, client := newTestConn(t)

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		// Include control characters (0x01, 0x07) and tab (0x09).
		// Tab is filtered (REQ-USE-6); other control chars are also filtered.
		_, _ = client.Write([]byte("he\x01ll\x07o\tworld\r\n"))
	}()

	line, err := conn.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "helloworld", line)
}

func TestConn_ReadLine_WithIAC(t *testing.T) {
	conn, client := newTestConn(t)

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		// Send IAC WILL ECHO followed by actual text
		_, _ = client.Write([]byte{IAC, WILL, OptEcho, 'h', 'i', '\r', '\n'})
	}()

	line, err := conn.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "hi", line)
}

func TestConn_ReadLine_WithIAC_SubNegotiation(t *testing.T) {
	conn, client := newTestConn(t)

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		data := []byte{IAC, SB, 24, 0, 'v', 't', '1', '0', '0', IAC, SE}
		data = append(data, []byte("ok\r\n")...)
		_, _ = client.Write(data)
	}()

	line, err := conn.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "ok", line)
}

func TestConn_Negotiate(t *testing.T) {
	conn, client := newTestConn(t)

	go func() {
		err := conn.Negotiate()
		assert.NoError(t, err)
	}()

	buf := make([]byte, 256)
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := client.Read(buf)
	require.NoError(t, err)

	expected := []byte{
		IAC, WILL, OptSuppressGoAhead,
		IAC, DO, OptNAWS,
		IAC, DONT, OptLinemode,
	}
	assert.Equal(t, expected, buf[:n])
}

func TestConn_Dimensions_DefaultZero(t *testing.T) {
	conn, _ := newTestConn(t)
	w, h := conn.Dimensions()
	assert.Equal(t, 0, w)
	assert.Equal(t, 0, h)
}

func TestConn_SplitScreen_DefaultFalse(t *testing.T) {
	conn, _ := newTestConn(t)
	assert.False(t, conn.IsSplitScreen())
}

// TestConn_ReadLine_NAWSUpdatesWidthHeight verifies that a NAWS subnegotiation
// embedded in the input stream sets Width and Height on the Conn.
func TestConn_ReadLine_NAWSUpdatesWidthHeight(t *testing.T) {
	conn, client := newTestConn(t)

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		// NAWS: 80 wide (0x00, 0x50), 24 high (0x00, 0x18)
		naws := []byte{IAC, SB, OptNAWS, 0x00, 0x50, 0x00, 0x18, IAC, SE}
		naws = append(naws, []byte("ok\r\n")...)
		_, _ = client.Write(naws)
	}()

	line, err := conn.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "ok", line)

	w, h := conn.Dimensions()
	assert.Equal(t, 80, w)
	assert.Equal(t, 24, h)
}

// TestConn_ReadLine_NAWS_SetsWidthHeight_Property verifies any valid 16-bit
// NAWS dimensions are stored correctly.
func TestConn_ReadLine_NAWS_SetsWidthHeight_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		w := rapid.IntRange(10, 300).Draw(rt, "w")
		h := rapid.IntRange(5, 100).Draw(rt, "h")

		conn, client := newTestConn(t)

		go func() {
			_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
			naws := []byte{
				IAC, SB, OptNAWS,
				byte(w >> 8), byte(w & 0xFF),
				byte(h >> 8), byte(h & 0xFF),
				IAC, SE,
			}
			naws = append(naws, []byte("x\r\n")...)
			_, _ = client.Write(naws)
		}()

		_, err := conn.ReadLine()
		require.NoError(t, err)

		gw, gh := conn.Dimensions()
		assert.Equal(t, w, gw)
		assert.Equal(t, h, gh)
	})
}

// TestConn_ReadLine_UpArrow verifies that \033[A returns the sentinel "\x00UP".
func TestConn_ReadLine_UpArrow(t *testing.T) {
	conn, client := newTestConn(t)

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, _ = client.Write([]byte{0x1B, '[', 'A'})
		client.Close()
	}()

	line, err := conn.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "\x00UP", line)
}

// TestConn_ReadLine_DownArrow verifies that \033[B returns the sentinel "\x00DOWN".
func TestConn_ReadLine_DownArrow(t *testing.T) {
	conn, client := newTestConn(t)

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, _ = client.Write([]byte{0x1B, '[', 'B'})
		client.Close()
	}()

	line, err := conn.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "\x00DOWN", line)
}

// TestConn_ReadLine_UnknownEscapeIgnored verifies that unknown CSI sequences
// are swallowed and reading continues.
func TestConn_ReadLine_UnknownEscapeIgnored(t *testing.T) {
	conn, client := newTestConn(t)

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		// \033[C (right arrow) then actual text
		_, _ = client.Write([]byte{0x1B, '[', 'C', 'h', 'i', '\r', '\n'})
	}()

	line, err := conn.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "hi", line)
}

// TestConn_AwaitNAWS_SetsWidthHeight verifies that AwaitNAWS processes the NAWS
// subnegotiation and returns true when dimensions are received within the timeout.
func TestConn_AwaitNAWS_SetsWidthHeight(t *testing.T) {
	conn, client := newTestConn(t)

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		naws := []byte{IAC, SB, OptNAWS, 0x00, 0x50, 0x00, 0x18, IAC, SE}
		_, _ = client.Write(naws)
	}()

	got := conn.AwaitNAWS(500 * time.Millisecond)
	assert.True(t, got)
	w, h := conn.Dimensions()
	assert.Equal(t, 80, w)
	assert.Equal(t, 24, h)
}

// TestConn_AwaitNAWS_TimesOut verifies that AwaitNAWS returns false when
// no NAWS arrives within the timeout.
func TestConn_AwaitNAWS_TimesOut(t *testing.T) {
	conn, _ := newTestConn(t)

	got := conn.AwaitNAWS(50 * time.Millisecond)
	assert.False(t, got)
	w, h := conn.Dimensions()
	assert.Equal(t, 0, w)
	assert.Equal(t, 0, h)
}

// TestConn_ResizeCh_FiresOnNAWS verifies that after ReadLine processes a NAWS
// subnegotiation, the resize channel receives a signal.
func TestConn_ResizeCh_FiresOnNAWS(t *testing.T) {
	conn, client := newTestConn(t)

	resizeCh := conn.ResizeCh()

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		naws := []byte{IAC, SB, OptNAWS, 0x00, 0x50, 0x00, 0x18, IAC, SE}
		naws = append(naws, []byte("x\r\n")...)
		_, _ = client.Write(naws)
	}()

	_, err := conn.ReadLine()
	require.NoError(t, err)

	select {
	case <-resizeCh:
		// good — signal received
	case <-time.After(500 * time.Millisecond):
		t.Fatal("resize channel did not fire after NAWS")
	}
}

func TestTryReadEscapeSeq_PgUp(t *testing.T) {
	raw := &bytes.Buffer{}
	raw.Write([]byte{'[', '5', '~'})
	c := &Conn{reader: bufio.NewReader(raw)}
	got := c.tryReadEscapeSeq()
	assert.Equal(t, "\x00PGUP", got)
}

func TestTryReadEscapeSeq_PgDn(t *testing.T) {
	raw := &bytes.Buffer{}
	raw.Write([]byte{'[', '6', '~'})
	c := &Conn{reader: bufio.NewReader(raw)}
	got := c.tryReadEscapeSeq()
	assert.Equal(t, "\x00PGDN", got)
}

func TestTryReadEscapeSeq_UnrecognizedDigitSwallowed(t *testing.T) {
	// ESC [ 3 ~ — unrecognized digit sequence; all bytes including ~ must be consumed.
	data := []byte{'[', '3', '~'}
	raw := &bytes.Buffer{}
	raw.Write(data)
	br := bufio.NewReader(raw)
	c := &Conn{reader: br}
	got := c.tryReadEscapeSeq()
	assert.Equal(t, "", got)
	// Verify ~ was consumed: attempting to read another byte should EOF.
	_, err := br.ReadByte()
	assert.ErrorIs(t, err, io.EOF)
}

func TestTryReadEscapeSeq_MultiDigitSwallowed(t *testing.T) {
	// ESC [ 1 5 ~ (F5 on VT220) — two digit parameters, must consume all bytes.
	raw := &bytes.Buffer{}
	raw.Write([]byte{'[', '1', '5', '~'})
	br := bufio.NewReader(raw)
	c := &Conn{reader: br}
	got := c.tryReadEscapeSeq()
	assert.Equal(t, "", got)
	// Verify ~ was consumed: reader should be empty.
	_, err := br.ReadByte()
	assert.ErrorIs(t, err, io.EOF)
}

func TestTryReadEscapeSeq_ArrowsUnchanged(t *testing.T) {
	for _, tc := range []struct {
		in   byte
		want string
	}{
		{'A', "\x00UP"}, {'B', "\x00DOWN"},
	} {
		raw := &bytes.Buffer{}
		raw.Write([]byte{'[', tc.in})
		c := &Conn{reader: bufio.NewReader(raw)}
		assert.Equal(t, tc.want, c.tryReadEscapeSeq())
	}
}

// --- Property tests ---

// Property: FilterIAC on input without any IAC bytes returns the input unchanged.
func TestPropertyFilterIAC_NoIACBytesPassThrough(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		length := rapid.IntRange(0, 200).Draw(t, "length")
		input := make([]byte, length)
		for i := range input {
			input[i] = byte(rapid.IntRange(0, 254).Draw(t, "byte"))
		}
		result := FilterIAC(input)
		assert.Equal(t, input, result, "input without IAC bytes should pass through unchanged")
	})
}

// Property: FilterIAC output never contains IAC followed by a Telnet command byte.
// A bare 0xFF in output is valid (it represents a literal 0xFF from an escaped IAC IAC).
func TestPropertyFilterIAC_OutputHasNoIACCommands(t *testing.T) {
	commandBytes := map[byte]bool{
		WILL: true, WONT: true, DO: true, DONT: true,
		SB: true, SE: true, NOP: true, GA: true,
	}
	rapid.Check(t, func(t *rapid.T) {
		length := rapid.IntRange(0, 100).Draw(t, "length")
		input := make([]byte, length)
		for i := range input {
			input[i] = byte(rapid.IntRange(0, 255).Draw(t, "byte"))
		}
		result := FilterIAC(input)
		for i := 0; i < len(result)-1; i++ {
			if result[i] == IAC {
				next := result[i+1]
				assert.False(t, commandBytes[next],
					"output should not contain IAC followed by command byte 0x%02x at position %d", next, i)
			}
		}
	})
}

// Property: FilterIAC output length is always <= input length.
func TestPropertyFilterIAC_OutputNeverLongerThanInput(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		length := rapid.IntRange(0, 200).Draw(t, "length")
		input := make([]byte, length)
		for i := range input {
			input[i] = byte(rapid.IntRange(0, 255).Draw(t, "byte"))
		}
		result := FilterIAC(input)
		assert.LessOrEqual(t, len(result), len(input),
			"filtered output should never be longer than input")
	})
}

func TestConsoleBuf_RingTruncatesAt1000(t *testing.T) {
	c := &Conn{}
	for i := 0; i < 1100; i++ {
		c.appendConsoleLine(fmt.Sprintf("line %d", i))
	}
	c.mu.Lock()
	n := len(c.consoleBuf)
	c.mu.Unlock()
	assert.Equal(t, consoleBufMax, n)
	c.mu.Lock()
	first := c.consoleBuf[0]
	c.mu.Unlock()
	assert.Equal(t, "line 100", first)
}

func TestConsoleBuf_PendingNewIncrementsWhenScrolled(t *testing.T) {
	c := &Conn{}
	c.mu.Lock()
	c.scrollOffset = 5
	c.mu.Unlock()
	c.appendConsoleLine("msg")
	c.mu.Lock()
	pn := c.pendingNew
	c.mu.Unlock()
	assert.Equal(t, 1, pn)
}

func TestConsoleBuf_PendingNewNotIncrementedWhenLive(t *testing.T) {
	c := &Conn{}
	c.appendConsoleLine("msg")
	c.mu.Lock()
	pn := c.pendingNew
	c.mu.Unlock()
	assert.Equal(t, 0, pn)
}

func TestConsoleSlice_LiveView(t *testing.T) {
	buf := make([]string, 50)
	for i := range buf {
		buf[i] = fmt.Sprintf("line-%d", i)
	}
	c := &Conn{consoleBuf: buf, height: 24, width: 80}
	// consoleHeight = 24 - 10 - 2 = 12
	lines := c.consoleSlice()
	assert.Equal(t, 12, len(lines))
	assert.Equal(t, "line-49", lines[len(lines)-1])
	assert.Equal(t, "line-38", lines[0])
}

func TestConsoleSlice_ScrolledBack(t *testing.T) {
	buf := make([]string, 50)
	for i := range buf {
		buf[i] = fmt.Sprintf("line-%d", i)
	}
	c := &Conn{consoleBuf: buf, height: 24, width: 80}
	c.mu.Lock()
	c.scrollOffset = 12
	c.mu.Unlock()
	lines := c.consoleSlice()
	assert.Equal(t, 12, len(lines))
	assert.Equal(t, "line-37", lines[len(lines)-1])
	assert.Equal(t, "line-26", lines[0])
}

func TestConsoleSlice_FewerLinesThanHeight(t *testing.T) {
	c := &Conn{height: 24, width: 80}
	c.consoleBuf = []string{"a", "b", "c"}
	lines := c.consoleSlice()
	assert.Equal(t, 3, len(lines))
	assert.Equal(t, "c", lines[len(lines)-1])
}

func TestScrollUp_IncrementsOffset(t *testing.T) {
	c := &Conn{height: 24, width: 80}
	for i := 0; i < 100; i++ {
		c.consoleBuf = append(c.consoleBuf, fmt.Sprintf("line-%d", i))
	}
	c.scrollUpState()
	c.mu.Lock()
	off := c.scrollOffset
	c.mu.Unlock()
	// consoleHeight = 24 - 10 - 2 = 12
	assert.Equal(t, 12, off)
}

func TestScrollDown_DecrementsOffset(t *testing.T) {
	c := &Conn{height: 24, width: 80}
	for i := 0; i < 100; i++ {
		c.consoleBuf = append(c.consoleBuf, fmt.Sprintf("line-%d", i))
	}
	c.mu.Lock()
	c.scrollOffset = 12
	c.pendingNew = 5
	c.mu.Unlock()
	c.scrollDownState()
	c.mu.Lock()
	off := c.scrollOffset
	pn := c.pendingNew
	c.mu.Unlock()
	assert.Equal(t, 0, off)
	assert.Equal(t, 0, pn) // pendingNew cleared when returning to live
}

func TestScrollUp_ClampsAtBufferBound(t *testing.T) {
	c := &Conn{height: 24, width: 80}
	for i := 0; i < 5; i++ {
		c.consoleBuf = append(c.consoleBuf, fmt.Sprintf("line-%d", i))
	}
	c.scrollUpState()
	c.mu.Lock()
	off := c.scrollOffset
	c.mu.Unlock()
	assert.Equal(t, 5, off) // clamped to len(buf)=5, not consoleHeight=12
}

func TestScrollDown_ClampsAtZero(t *testing.T) {
	c := &Conn{height: 24, width: 80}
	c.scrollDownState()
	c.mu.Lock()
	off := c.scrollOffset
	c.mu.Unlock()
	assert.Equal(t, 0, off)
}

func TestScrollDown_PartialPage(t *testing.T) {
	// scrollOffset = 6, consoleHeight = 12 → scrollDown clamps to 0
	c := &Conn{height: 24, width: 80}
	c.mu.Lock()
	c.scrollOffset = 6
	c.pendingNew = 3
	c.mu.Unlock()
	c.scrollDownState()
	c.mu.Lock()
	off := c.scrollOffset
	pn := c.pendingNew
	c.mu.Unlock()
	assert.Equal(t, 0, off)
	assert.Equal(t, 0, pn)
}

func TestSnapToLive_ClearsScrollAndPending(t *testing.T) {
	c := &Conn{height: 24, width: 80}
	c.mu.Lock()
	c.scrollOffset = 24
	c.pendingNew = 7
	c.mu.Unlock()
	c.snapToLiveState()
	c.mu.Lock()
	off := c.scrollOffset
	pn := c.pendingNew
	c.mu.Unlock()
	assert.Equal(t, 0, off)
	assert.Equal(t, 0, pn)
}

func TestScrollUpLine_IncrementsOffsetByOne(t *testing.T) {
	c := &Conn{height: 24, width: 80}
	for i := 0; i < 100; i++ {
		c.consoleBuf = append(c.consoleBuf, fmt.Sprintf("line-%d", i))
	}
	c.scrollUpLineState()
	c.mu.Lock()
	off := c.scrollOffset
	c.mu.Unlock()
	assert.Equal(t, 1, off)
}

func TestScrollDownLine_DecrementsOffsetByOne(t *testing.T) {
	c := &Conn{height: 24, width: 80}
	for i := 0; i < 100; i++ {
		c.consoleBuf = append(c.consoleBuf, fmt.Sprintf("line-%d", i))
	}
	c.mu.Lock()
	c.scrollOffset = 5
	c.pendingNew = 3
	c.mu.Unlock()
	c.scrollDownLineState()
	c.mu.Lock()
	off := c.scrollOffset
	pn := c.pendingNew
	c.mu.Unlock()
	assert.Equal(t, 4, off)
	assert.Equal(t, 3, pn) // pendingNew not cleared until live
}

func TestScrollDownLine_ClearsLiveView(t *testing.T) {
	c := &Conn{height: 24, width: 80}
	c.mu.Lock()
	c.scrollOffset = 1
	c.pendingNew = 2
	c.mu.Unlock()
	c.scrollDownLineState()
	c.mu.Lock()
	off := c.scrollOffset
	pn := c.pendingNew
	c.mu.Unlock()
	assert.Equal(t, 0, off)
	assert.Equal(t, 0, pn)
}

func TestScrollUpLine_ClampsAtBufferBound(t *testing.T) {
	c := &Conn{height: 24, width: 80}
	c.consoleBuf = []string{"a", "b"}
	c.mu.Lock()
	c.scrollOffset = 2
	c.mu.Unlock()
	c.scrollUpLineState()
	c.mu.Lock()
	off := c.scrollOffset
	c.mu.Unlock()
	assert.Equal(t, 2, off) // clamped to len(buf)
}

func TestIsScrolledBack(t *testing.T) {
	c := &Conn{}
	assert.False(t, c.IsScrolledBack())
	c.mu.Lock()
	c.scrollOffset = 3
	c.mu.Unlock()
	assert.True(t, c.IsScrolledBack())
}

func TestConn_History_AppendAndNavigate(t *testing.T) {
	c := &Conn{}
	c.AppendHistory("look")
	c.AppendHistory("north")
	c.AppendHistory("inventory")

	got, ok := c.HistoryUp()
	require.True(t, ok)
	assert.Equal(t, "inventory", got)

	got, ok = c.HistoryUp()
	require.True(t, ok)
	assert.Equal(t, "north", got)

	got, ok = c.HistoryUp()
	require.True(t, ok)
	assert.Equal(t, "look", got)

	// At oldest entry, HistoryUp is a no-op
	got2, ok2 := c.HistoryUp()
	assert.False(t, ok2)
	assert.Equal(t, "", got2)

	// HistoryDown from oldest moves forward
	got, ok = c.HistoryDown()
	require.True(t, ok)
	assert.Equal(t, "north", got)

	got, ok = c.HistoryDown()
	require.True(t, ok)
	assert.Equal(t, "inventory", got)

	// HistoryDown at live position returns "", false
	got, ok = c.HistoryDown()
	assert.False(t, ok)
	assert.Equal(t, "", got)
}

func TestConn_History_ResetOnSubmit(t *testing.T) {
	c := &Conn{}
	c.AppendHistory("look")
	c.AppendHistory("north")
	_, _ = c.HistoryUp()
	c.AppendHistory("inventory")
	got, ok := c.HistoryUp()
	require.True(t, ok)
	assert.Equal(t, "inventory", got)
}

func TestConn_History_RingOverflow(t *testing.T) {
	c := &Conn{}
	// Append 110 commands (10 over the 100-entry max)
	for i := range 110 {
		c.AppendHistory(fmt.Sprintf("cmd%d", i))
	}
	// Only the last 100 should be retained
	// HistoryUp 100 times should succeed; 101st should fail
	for i := range 100 {
		_, ok := c.HistoryUp()
		require.True(t, ok, "HistoryUp should succeed at step %d", i)
	}
	_, ok := c.HistoryUp()
	assert.False(t, ok, "HistoryUp beyond 100 entries should return false")
}

func TestProperty_History_ReverseOrder(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 20).Draw(rt, "n")
		c := &Conn{}
		cmds := make([]string, n)
		for i := range n {
			// Generate non-empty printable strings as realistic commands
			cmds[i] = rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(rt, fmt.Sprintf("cmd%d", i))
			c.AppendHistory(cmds[i])
		}
		for i := n - 1; i >= 0; i-- {
			got, ok := c.HistoryUp()
			if !ok {
				rt.Fatalf("HistoryUp returned false at index %d", i)
			}
			if got != cmds[i] {
				rt.Fatalf("at index %d: want %q, got %q", i, cmds[i], got)
			}
		}
	})
}

func TestTryReadEscapeSeq_ShiftArrows(t *testing.T) {
	tests := []struct {
		input    []byte
		sentinel string
	}{
		{[]byte{'[', '1', ';', '2', 'A'}, "\x00SHIFT_UP"},
		{[]byte{'[', '1', ';', '2', 'B'}, "\x00SHIFT_DOWN"},
		{[]byte{'[', 'A'}, "\x00UP"},
		{[]byte{'[', 'B'}, "\x00DOWN"},
	}
	for _, tt := range tests {
		c := &Conn{reader: bufio.NewReader(bytes.NewReader(tt.input))}
		got := c.tryReadEscapeSeq()
		assert.Equal(t, tt.sentinel, got, "input %v", tt.input)
	}
}

func TestIntegration_ConsoleScroll(t *testing.T) {
	c := &Conn{height: 24, width: 80}

	// Write 200 lines into the buffer.
	for i := 0; i < 200; i++ {
		c.appendConsoleLine(fmt.Sprintf("line-%d", i))
	}

	// consoleHeight = 24 - 10 - 2 = 12
	// Scroll up one page.
	c.scrollUpState()
	c.mu.Lock()
	off := c.scrollOffset
	c.mu.Unlock()
	assert.Equal(t, 12, off)

	// consoleSlice should show lines 176-187
	// end = 200 - 12 = 188; start = 188 - 12 = 176
	slice := c.consoleSlice()
	assert.Equal(t, 12, len(slice))
	assert.Equal(t, "line-176", slice[0])
	assert.Equal(t, "line-187", slice[len(slice)-1])

	// Append more lines while scrolled — pendingNew increments.
	c.appendConsoleLine("new-0")
	c.appendConsoleLine("new-1")
	c.mu.Lock()
	pn := c.pendingNew
	c.mu.Unlock()
	assert.Equal(t, 2, pn)

	// Scroll down to live — pendingNew cleared.
	c.scrollDownState()
	c.mu.Lock()
	off = c.scrollOffset
	pn = c.pendingNew
	c.mu.Unlock()
	assert.Equal(t, 0, off)
	assert.Equal(t, 0, pn)

	// consoleSlice at live shows the 12 most recent lines (196-201 = line-196..line-199, new-0, new-1)
	slice = c.consoleSlice()
	assert.Equal(t, 12, len(slice))
	assert.Equal(t, "new-1", slice[len(slice)-1])
}

// --- BUG-7: ClearConsoleBuf ---

// TestClearConsoleBuf_ResetsBufferAndOffset verifies that ClearConsoleBuf
// sets consoleBuf to nil, scrollOffset to 0, and pendingNew to 0.
func TestClearConsoleBuf_ResetsBufferAndOffset(t *testing.T) {
	c := &Conn{}
	for i := 0; i < 50; i++ {
		c.appendConsoleLine(fmt.Sprintf("line-%d", i))
	}
	c.mu.Lock()
	c.scrollOffset = 10
	c.pendingNew = 3
	c.mu.Unlock()

	c.ClearConsoleBuf()

	c.mu.Lock()
	buf := c.consoleBuf
	off := c.scrollOffset
	pn := c.pendingNew
	c.mu.Unlock()

	assert.Nil(t, buf, "consoleBuf should be nil after ClearConsoleBuf")
	assert.Equal(t, 0, off, "scrollOffset should be 0 after ClearConsoleBuf")
	assert.Equal(t, 0, pn, "pendingNew should be 0 after ClearConsoleBuf")
}

// TestClearConsoleBuf_IdempotentOnEmpty verifies that calling ClearConsoleBuf
// on an already-empty Conn is safe and leaves state zeroed.
func TestClearConsoleBuf_IdempotentOnEmpty(t *testing.T) {
	c := &Conn{}
	c.ClearConsoleBuf()

	c.mu.Lock()
	buf := c.consoleBuf
	off := c.scrollOffset
	pn := c.pendingNew
	c.mu.Unlock()

	assert.Nil(t, buf)
	assert.Equal(t, 0, off)
	assert.Equal(t, 0, pn)
}

// --- BUG-13: ReadLineSplit seeds from inputBuf ---

// TestReadLineSplit_SeedsFromInputBuf verifies that when inputBuf is set before
// ReadLineSplit is called, pressing Enter submits the seeded content.
func TestReadLineSplit_SeedsFromInputBuf(t *testing.T) {
	conn, client := newTestConn(t)
	conn.EnableSplitScreen()
	conn.SetInputBuf("recall")

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		// Client presses Enter immediately — should submit the seeded "recall"
		_, _ = client.Write([]byte("\r\n"))
	}()

	line, err := conn.ReadLineSplit()
	require.NoError(t, err)
	assert.Equal(t, "recall", line)
}

// TestReadLineSplit_SeedsFromInputBufThenClearsOnSubmit verifies that after
// submitting the seeded value, inputBuf is cleared.
func TestReadLineSplit_SeedsFromInputBufThenClearsOnSubmit(t *testing.T) {
	conn, client := newTestConn(t)
	conn.EnableSplitScreen()
	conn.SetInputBuf("north")

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, _ = client.Write([]byte("\r\n"))
	}()

	line, err := conn.ReadLineSplit()
	require.NoError(t, err)
	assert.Equal(t, "north", line)

	conn.mu.Lock()
	remaining := conn.inputBuf
	conn.mu.Unlock()
	assert.Equal(t, "", remaining, "inputBuf should be cleared after ReadLineSplit returns")
}

// TestReadLineSplit_SeedPlusTypedChars verifies that typed characters are appended
// after the seeded inputBuf content.
func TestReadLineSplit_SeedPlusTypedChars(t *testing.T) {
	conn, client := newTestConn(t)
	conn.EnableSplitScreen()
	conn.SetInputBuf("go ")

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		// Type "north" then Enter
		_, _ = client.Write([]byte("north\r\n"))
	}()

	line, err := conn.ReadLineSplit()
	require.NoError(t, err)
	assert.Equal(t, "go north", line)
}

// --- HeadlessConn tests ---

func newTestHeadlessConn(t *testing.T) (*Conn, net.Conn) {
	t.Helper()
	client, server := net.Pipe()
	conn := NewHeadlessConn(server, 2*time.Second, 2*time.Second)
	t.Cleanup(func() {
		client.Close()
		conn.Close()
	})
	return conn, client
}

func TestHeadlessConn_InitScreen_IsNoop(t *testing.T) {
	conn, client := newTestHeadlessConn(t)

	// InitScreen should not write anything to the connection.
	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 256)
		_ = client.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, _ := client.Read(buf)
		done <- buf[:n]
	}()

	err := conn.InitScreen()
	assert.NoError(t, err)

	received := <-done
	assert.Empty(t, received, "InitScreen on headless conn must write nothing")
}

func TestHeadlessConn_WriteRoom_PlainText(t *testing.T) {
	conn, client := newTestHeadlessConn(t)

	content := "\033[1mThe Dark Forest\033[0m\nA shadowy glade surrounds you.\nExits: north south"

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		reader := bufio.NewReader(client)
		_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
		// Read until we see the blank line after room content (two consecutive \r\n).
		for {
			line, err := reader.ReadString('\n')
			buf.WriteString(line)
			if err != nil {
				break
			}
			if strings.Contains(buf.String(), "\r\n\r\n") {
				break
			}
		}
		done <- buf.String()
	}()

	err := conn.WriteRoom(content)
	assert.NoError(t, err)

	received := <-done
	assert.NotContains(t, received, "\033[", "WriteRoom on headless conn must not emit ANSI")
	assert.Contains(t, received, "The Dark Forest")
	assert.Contains(t, received, "A shadowy glade surrounds you.")
	assert.Contains(t, received, "Exits: north south")
}

func TestHeadlessConn_WriteConsole_PlainText(t *testing.T) {
	conn, client := newTestHeadlessConn(t)

	msg := "\033[32mYou strike the guard for 12 damage.\033[0m"

	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 512)
		_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _ := client.Read(buf)
		done <- string(buf[:n])
	}()

	err := conn.WriteConsole(msg)
	assert.NoError(t, err)

	received := <-done
	assert.NotContains(t, received, "\033[", "WriteConsole on headless conn must not emit ANSI")
	assert.Contains(t, received, "You strike the guard for 12 damage.")
}

func TestHeadlessConn_WritePrompt_PlainPrompt(t *testing.T) {
	conn, client := newTestHeadlessConn(t)

	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 64)
		_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _ := client.Read(buf)
		done <- string(buf[:n])
	}()

	err := conn.WritePromptSplit("> ")
	assert.NoError(t, err)

	received := <-done
	assert.NotContains(t, received, "\033[", "WritePromptSplit on headless conn must not emit ANSI")
	assert.Equal(t, "> ", received)
}

func TestHeadlessConn_WriteConsole_NeverEmitsANSI(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random string that might contain ANSI-like content
		msg := rapid.String().Draw(rt, "msg")

		client, server := net.Pipe()
		conn := NewHeadlessConn(server, 2*time.Second, 2*time.Second)
		defer client.Close()
		defer conn.Close()

		done := make(chan string, 1)
		go func() {
			buf := make([]byte, 4096)
			_ = client.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, _ := client.Read(buf)
			done <- string(buf[:n])
		}()

		_ = conn.WriteConsole(msg)

		received := <-done
		assert.NotContains(rt, received, "\033[", "WriteConsole must never emit ANSI escape sequences")
	})
}

// --- Tab key (REQ-USE-5, REQ-USE-6) ---

// TestConn_TabKey_InvokesTabCompleter verifies that pressing tab during ReadLineSplit
// calls TabCompleter with the current buffer prefix and does not unblock ReadLine.
// REQ-USE-5.
func TestConn_TabKey_InvokesTabCompleter(t *testing.T) {
	conn, client := newTestConn(t)

	var gotPrefix string
	done := make(chan struct{})
	conn.TabCompleter = func(prefix string) {
		gotPrefix = prefix
		close(done)
	}

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		// Write "use" then tab then newline (to unblock ReadLineSplit).
		_, _ = client.Write([]byte("use\x09\r\n"))
	}()

	_, err := conn.ReadLineSplit()
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("TabCompleter was not called within timeout")
	}
	assert.Equal(t, "use", gotPrefix)
}

// TestConn_TabKey_NilCompleter_DoesNotPanic verifies that pressing tab when
// TabCompleter is nil does not panic. REQ-USE-5.
func TestConn_TabKey_NilCompleter_DoesNotPanic(t *testing.T) {
	conn, client := newTestConn(t)
	// TabCompleter is nil by default.

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, _ = client.Write([]byte("\x09\r\n"))
	}()

	assert.NotPanics(t, func() {
		_, _ = conn.ReadLineSplit()
	})
}

// TestConn_TabKey_DoesNotModifyBuffer verifies that tab is not appended to the
// input buffer in ReadLineSplit. REQ-USE-6.
func TestConn_TabKey_DoesNotModifyBuffer(t *testing.T) {
	conn, client := newTestConn(t)
	conn.TabCompleter = func(prefix string) {}

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, _ = client.Write([]byte("ab\x09cd\r\n"))
	}()

	line, err := conn.ReadLineSplit()
	require.NoError(t, err)
	// Tab must not appear in the returned line.
	assert.Equal(t, "abcd", line)
	assert.NotContains(t, line, "\t")
}

// TestConn_ReadLine_TabKey_DoesNotModifyBuffer verifies that tab is not appended
// to the input buffer in ReadLine. REQ-USE-6.
func TestConn_ReadLine_TabKey_DoesNotModifyBuffer(t *testing.T) {
	conn, client := newTestConn(t)
	conn.TabCompleter = func(prefix string) {}

	go func() {
		_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, _ = client.Write([]byte("ab\x09cd\r\n"))
	}()

	line, err := conn.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "abcd", line)
	assert.NotContains(t, line, "\t")
}
