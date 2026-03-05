package telnet

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"sync"
	"time"
)

// Telnet IAC (Interpret As Command) constants per RFC 854.
const (
	IAC  byte = 255 // Interpret As Command
	DONT byte = 254
	DO   byte = 253
	WONT byte = 252
	WILL byte = 251
	SB   byte = 250 // Sub-negotiation Begin
	SE   byte = 240 // Sub-negotiation End
	NOP  byte = 241
	GA   byte = 249 // Go Ahead

	// Telnet options
	OptEcho            byte = 1
	OptSuppressGoAhead byte = 3
	OptNAWS            byte = 31 // Negotiate About Window Size, RFC 1073
	OptLinemode        byte = 34
)

// Conn wraps a TCP connection with Telnet protocol handling.
// It filters IAC sequences from input and provides line-based reading.
type Conn struct {
	raw    net.Conn
	reader *bufio.Reader
	mu     sync.Mutex

	readTimeout  time.Duration
	writeTimeout time.Duration

	// Split-screen state (guarded by mu)
	width       int
	height      int
	splitScreen bool
	inputBuf    string

	// resizeCh is signalled (non-blocking) whenever NAWS updates width/height.
	resizeCh chan struct{}
}

// NewConn wraps a raw TCP connection with Telnet protocol handling.
//
// Precondition: raw must be a valid, open network connection.
// Postcondition: Returns a Conn ready for reading and writing.
func NewConn(raw net.Conn, readTimeout, writeTimeout time.Duration) *Conn {
	return &Conn{
		raw:          raw,
		reader:       bufio.NewReaderSize(raw, 4096),
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
		resizeCh:     make(chan struct{}, 1),
	}
}

// Negotiate sends initial Telnet option negotiations.
// We request the client to suppress go-ahead and let us handle echo.
//
// Postcondition: Negotiation bytes are written to the connection.
func (c *Conn) Negotiate() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	negotiations := []byte{
		IAC, WILL, OptSuppressGoAhead,
		IAC, DO, OptNAWS,
	}

	if c.writeTimeout > 0 {
		_ = c.raw.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}
	_, err := c.raw.Write(negotiations)
	return err
}

// Dimensions returns the negotiated terminal width and height.
// Returns (0, 0) before NAWS subnegotiation is received.
func (c *Conn) Dimensions() (width, height int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.width, c.height
}

// IsSplitScreen reports whether the connection is in split-screen mode.
func (c *Conn) IsSplitScreen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.splitScreen
}

// ResizeCh returns a channel that receives a value whenever terminal dimensions change.
// The channel is buffered (capacity 1) so the signal is non-blocking.
func (c *Conn) ResizeCh() <-chan struct{} {
	return c.resizeCh
}

// ReadLine reads a single line of input, filtering Telnet IAC sequences.
// The returned line does not include the trailing \r\n.
//
// Postcondition: Returns the next line of text input, or an error (including io.EOF).
func (c *Conn) ReadLine() (string, error) {
	if c.readTimeout > 0 {
		_ = c.raw.SetReadDeadline(time.Now().Add(c.readTimeout))
	}

	var line bytes.Buffer
	for {
		b, err := c.reader.ReadByte()
		if err != nil {
			return line.String(), err
		}

		if b == IAC {
			if err := c.handleIAC(); err != nil {
				return line.String(), err
			}
			continue
		}

		if b == '\n' {
			break
		}
		if b == '\r' {
			// Peek ahead — if next is \n, consume it
			next, err := c.reader.Peek(1)
			if err == nil && len(next) > 0 && next[0] == '\n' {
				_, _ = c.reader.ReadByte()
			}
			break
		}

		// Handle ESC: detect VT100 arrow key sequences
		if b == 0x1B {
			if sentinel := c.tryReadEscapeSeq(); sentinel != "" {
				return sentinel, nil
			}
			// Unknown or incomplete escape sequence — swallow and continue
			continue
		}

		// Filter control characters except tab
		if b < 32 && b != '\t' {
			continue
		}

		line.WriteByte(b)
	}

	return line.String(), nil
}

// tryReadEscapeSeq attempts to read a VT100 CSI escape sequence after ESC (0x1B)
// has been consumed. Returns a sentinel string if the sequence is a recognized
// arrow key, or "" to indicate an unrecognized sequence (all bytes consumed).
//
// Precondition: ESC byte has already been read from c.reader.
// Postcondition: The full CSI sequence ([ + final byte) has been consumed.
func (c *Conn) tryReadEscapeSeq() string {
	next, err := c.reader.ReadByte()
	if err != nil || next != '[' {
		// Bare ESC or read error — nothing more to consume
		return ""
	}
	final, err := c.reader.ReadByte()
	if err != nil {
		return ""
	}
	switch final {
	case 'A':
		return "\x00UP"
	case 'B':
		return "\x00DOWN"
	default:
		return ""
	}
}

// handleIAC processes a Telnet IAC sequence after the initial IAC byte
// has been read.
func (c *Conn) handleIAC() error {
	cmd, err := c.reader.ReadByte()
	if err != nil {
		return err
	}

	switch cmd {
	case WILL, WONT, DO, DONT:
		// These commands have one option byte following
		_, err := c.reader.ReadByte()
		return err
	case SB:
		// Sub-negotiation: read option byte, then data until IAC SE.
		opt, err := c.reader.ReadByte()
		if err != nil {
			return err
		}
		var subdata []byte
		for {
			b, err := c.reader.ReadByte()
			if err != nil {
				return err
			}
			if b == IAC {
				next, err := c.reader.ReadByte()
				if err != nil {
					return err
				}
				if next == SE {
					break
				}
				// IAC IAC inside SB = literal 0xFF; the byte after IAC is
				// still regular data and must not be discarded.
				subdata = append(subdata, 0xFF)
				if next != IAC {
					subdata = append(subdata, next)
				}
				continue
			}
			subdata = append(subdata, b)
		}
		// Parse NAWS: option 31, exactly 4 payload bytes (W-hi W-lo H-hi H-lo)
		if opt == OptNAWS && len(subdata) == 4 {
			w := int(subdata[0])<<8 | int(subdata[1])
			h := int(subdata[2])<<8 | int(subdata[3])
			c.mu.Lock()
			c.width = w
			c.height = h
			c.mu.Unlock()
			// Signal resize (non-blocking — capacity-1 buffer absorbs rapid resizes)
			select {
			case c.resizeCh <- struct{}{}:
			default:
			}
		}
	case IAC:
		// Escaped IAC (literal 0xFF) — we ignore it in text context
	default:
		// Other commands (NOP, GA, etc.) — ignore
	}
	return nil
}

// AwaitNAWS waits up to timeout for the client to send a NAWS subnegotiation.
// It reads and discards all incoming bytes until NAWS dimensions arrive or timeout.
// Returns true if non-zero dimensions were received before the timeout.
//
// Precondition: Negotiate() must already have been called (IAC DO NAWS was sent).
// Postcondition: conn.width and conn.height are set if NAWS was received.
func (c *Conn) AwaitNAWS(timeout time.Duration) bool {
	_ = c.raw.SetReadDeadline(time.Now().Add(timeout))
	defer func() { _ = c.raw.SetReadDeadline(time.Time{}) }()

	for {
		b, err := c.reader.ReadByte()
		if err != nil {
			return false
		}
		if b != IAC {
			continue
		}
		if err := c.handleIAC(); err != nil {
			return false
		}
		w, h := c.Dimensions()
		if w > 0 && h > 0 {
			return true
		}
	}
}

// ReadPassword reads a line of input with server-side echo suppression.
// It sends IAC WILL Echo before reading (client stops echoing) and
// IAC WONT Echo after (client resumes echoing), then writes a blank
// line so the cursor advances past the hidden input.
//
// Postcondition: Returns the trimmed input with echo restored.
func (c *Conn) ReadPassword() (string, error) {
	// Suppress client echo
	if err := c.sendEchoControl(WILL); err != nil {
		return "", err
	}

	line, err := c.ReadLine()

	// Always restore echo, even on error
	_ = c.sendEchoControl(WONT)
	// Advance the cursor past the invisible input
	_ = c.Write([]byte("\r\n"))

	return line, err
}

// sendEchoControl sends IAC <cmd> OptEcho to control client-side echo.
func (c *Conn) sendEchoControl(cmd byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.writeTimeout > 0 {
		_ = c.raw.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}
	_, err := c.raw.Write([]byte{IAC, cmd, OptEcho})
	return err
}

// WriteLine sends a line of text followed by \r\n to the client.
//
// Precondition: text should not contain trailing newline characters.
// Postcondition: text + \r\n is written to the connection.
func (c *Conn) WriteLine(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writeTimeout > 0 {
		_ = c.raw.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}
	_, err := fmt.Fprintf(c.raw, "%s\r\n", text)
	return err
}

// Write sends raw bytes to the client.
//
// Postcondition: The data is written to the connection.
func (c *Conn) Write(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writeTimeout > 0 {
		_ = c.raw.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}
	_, err := c.raw.Write(data)
	return err
}

// WritePrompt sends a prompt string without a trailing newline.
//
// Postcondition: The prompt text is written to the connection.
func (c *Conn) WritePrompt(prompt string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writeTimeout > 0 {
		_ = c.raw.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}
	_, err := fmt.Fprint(c.raw, prompt)
	return err
}

// Close closes the underlying TCP connection.
//
// Postcondition: The connection is closed and no longer usable.
func (c *Conn) Close() error {
	return c.raw.Close()
}

// RemoteAddr returns the remote network address of the client.
func (c *Conn) RemoteAddr() net.Addr {
	return c.raw.RemoteAddr()
}

// FilterIAC removes Telnet IAC sequences from raw input bytes.
// This is a pure function useful for testing and protocol parsing.
//
// Postcondition: Returns input with all IAC sequences removed.
func FilterIAC(input []byte) []byte {
	result := make([]byte, 0, len(input))
	i := 0
	for i < len(input) {
		if input[i] == IAC && i+1 < len(input) {
			cmd := input[i+1]
			switch cmd {
			case WILL, WONT, DO, DONT:
				// Skip IAC + cmd + option
				i += 3
				continue
			case SB:
				// Skip until IAC SE
				j := i + 2
				for j < len(input)-1 {
					if input[j] == IAC && input[j+1] == SE {
						j += 2
						break
					}
					j++
				}
				i = j
				continue
			case IAC:
				// Escaped 0xFF — discard in text-stripping context; emitting 0xFF could
				// form a false IAC+cmd sequence with the next byte in the output.
				i += 2
				continue
			default:
				// Other commands — skip IAC + cmd
				i += 2
				continue
			}
		}
		result = append(result, input[i])
		i++
	}
	return result
}
