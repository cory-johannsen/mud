package testutil

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// TelnetClient is a simple Telnet test client for integration testing.
type TelnetClient struct {
	conn   net.Conn
	reader *bufio.Reader
	t      *testing.T
}

// NewTelnetClient dials the given address and returns a test client.
//
// Precondition: addr must be a valid "host:port" string with a listening server.
// Postcondition: Returns a connected TelnetClient or fails the test.
func NewTelnetClient(t *testing.T, addr string) *TelnetClient {
	t.Helper()
	start := time.Now()

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("connecting to %s: %v [%s]", addr, err, time.Since(start))
	}

	t.Cleanup(func() {
		conn.Close()
	})

	client := &TelnetClient{
		conn:   conn,
		reader: bufio.NewReader(conn),
		t:      t,
	}

	t.Logf("telnet client connected to %s [%s]", addr, time.Since(start))
	return client
}

// ReadUntil reads data until the specified substring is found or timeout occurs.
// It returns all data read up to and including the match.
//
// Precondition: substr must be non-empty.
// Postcondition: Returns the accumulated output containing substr, or fails on timeout.
func (c *TelnetClient) ReadUntil(substr string, timeout time.Duration) string {
	c.t.Helper()
	_ = c.conn.SetReadDeadline(time.Now().Add(timeout))

	var buf strings.Builder
	tmp := make([]byte, 1024)
	for {
		n, err := c.conn.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
			if strings.Contains(buf.String(), substr) {
				return buf.String()
			}
		}
		if err != nil {
			c.t.Fatalf("reading until %q: got %q, error: %v", substr, buf.String(), err)
		}
	}
}

// Send writes a line of text to the server, appending \r\n.
//
// Precondition: text should not contain trailing newline characters.
// Postcondition: text + \r\n is written to the connection.
func (c *TelnetClient) Send(text string) {
	c.t.Helper()
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := fmt.Fprintf(c.conn, "%s\r\n", text)
	if err != nil {
		c.t.Fatalf("sending %q: %v", text, err)
	}
}

// Close closes the underlying connection.
func (c *TelnetClient) Close() {
	c.conn.Close()
}
