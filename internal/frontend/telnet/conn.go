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
	}

	if c.writeTimeout > 0 {
		_ = c.raw.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}
	_, err := c.raw.Write(negotiations)
	return err
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

		// Filter control characters except tab
		if b < 32 && b != '\t' {
			continue
		}

		line.WriteByte(b)
	}

	return line.String(), nil
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
		// Sub-negotiation: read until IAC SE
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
			}
		}
	case IAC:
		// Escaped IAC (literal 0xFF) — we ignore it in text context
	default:
		// Other commands (NOP, GA, etc.) — ignore
	}
	return nil
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
				// Escaped 0xFF — emit one 0xFF
				result = append(result, IAC)
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
