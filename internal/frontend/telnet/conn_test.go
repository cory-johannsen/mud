package telnet

import (
	"net"
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
	input := []byte{'a', IAC, IAC, 'b'}
	result := FilterIAC(input)
	assert.Equal(t, []byte{byte('a'), IAC, byte('b')}, result)
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
		// Include control characters (0x01, 0x07) but keep tab (0x09)
		_, _ = client.Write([]byte("he\x01ll\x07o\tworld\r\n"))
	}()

	line, err := conn.ReadLine()
	require.NoError(t, err)
	assert.Equal(t, "hello\tworld", line)
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
		IAC, WILL, OptEcho,
	}
	assert.Equal(t, expected, buf[:n])
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
		SB: true, NOP: true, GA: true,
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
