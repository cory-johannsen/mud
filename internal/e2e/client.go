// Package e2e provides the HeadlessClient and utilities for full-stack end-to-end tests.
// Tests in this package start ephemeral PostgreSQL, gameserver, and frontend subprocesses
// in TestMain and exercise game scenarios through the headless telnet port.
package e2e

import (
	"bufio"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

const defaultExpectTimeout = 5 * time.Second

// HeadlessClient is a thin TCP client for the headless telnet port.
//
// Precondition: created via Dial; safe for sequential use within a single test goroutine.
type HeadlessClient struct {
	conn   net.Conn
	reader *bufio.Reader
}

// Dial opens a TCP connection to addr and returns a ready HeadlessClient.
//
// Precondition: addr must be a valid "host:port" string.
// Postcondition: conn is open; returns non-nil error if dial fails.
func Dial(addr string) (*HeadlessClient, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return &HeadlessClient{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}, nil
}

// Send writes cmd followed by CRLF to the connection.
//
// Precondition: conn must be open.
// Postcondition: cmd+"\r\n" written to the wire; returns non-nil error on write failure.
func (c *HeadlessClient) Send(cmd string) error {
	_, err := fmt.Fprintf(c.conn, "%s\r\n", cmd)
	if err != nil {
		return fmt.Errorf("Send(%q): %w", cmd, err)
	}
	return nil
}

// ReadLine reads one line from the server with a deadline.
// timeout == 0 uses defaultExpectTimeout (5s).
//
// Postcondition: Returns the line stripped of trailing \r\n, or an error.
func (c *HeadlessClient) ReadLine(timeout time.Duration) (string, error) {
	if timeout == 0 {
		timeout = defaultExpectTimeout
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(timeout))
	line, err := c.reader.ReadString('\n')
	_ = c.conn.SetReadDeadline(time.Time{})
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// Expect reads lines until one contains pattern (substring match) or timeout elapses.
// timeout == 0 uses defaultExpectTimeout (5s).
//
// Postcondition: Returns nil on match; returns descriptive error on timeout.
func (c *HeadlessClient) Expect(pattern string, timeout time.Duration) error {
	if timeout == 0 {
		timeout = defaultExpectTimeout
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(remaining))
		line, err := c.reader.ReadString('\n')
		_ = c.conn.SetReadDeadline(time.Time{})
		if err != nil {
			if strings.Contains(line, pattern) {
				return nil
			}
			return fmt.Errorf("Expect(%q): timeout after %s: %w", pattern, timeout, err)
		}
		if strings.Contains(line, pattern) {
			return nil
		}
	}
	return fmt.Errorf("Expect(%q): pattern not found within %s", pattern, timeout)
}

// ExpectRegex reads lines until one matches pattern (regexp) or timeout elapses.
// timeout == 0 uses defaultExpectTimeout (5s).
//
// Postcondition: Returns nil on match; returns descriptive error on timeout or bad pattern.
func (c *HeadlessClient) ExpectRegex(pattern string, timeout time.Duration) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("ExpectRegex: invalid pattern %q: %w", pattern, err)
	}
	if timeout == 0 {
		timeout = defaultExpectTimeout
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(remaining))
		line, err := c.reader.ReadString('\n')
		_ = c.conn.SetReadDeadline(time.Time{})
		if err != nil {
			if re.MatchString(line) {
				return nil
			}
			return fmt.Errorf("ExpectRegex(%q): timeout after %s: %w", pattern, timeout, err)
		}
		if re.MatchString(line) {
			return nil
		}
	}
	return fmt.Errorf("ExpectRegex(%q): pattern not found within %s", pattern, timeout)
}

// ExpectRegexReturn reads lines until one matches pattern (regexp) or timeout elapses.
// Returns the matching line on success.
// timeout == 0 uses defaultExpectTimeout (5s).
//
// Postcondition: Returns the matching line and nil error on match; empty string and
// descriptive error on timeout or bad pattern.
func (c *HeadlessClient) ExpectRegexReturn(pattern string, timeout time.Duration) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("ExpectRegexReturn: invalid pattern %q: %w", pattern, err)
	}
	if timeout == 0 {
		timeout = defaultExpectTimeout
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(remaining))
		line, err := c.reader.ReadString('\n')
		_ = c.conn.SetReadDeadline(time.Time{})
		if err != nil {
			if re.MatchString(line) {
				return strings.TrimRight(line, "\r\n"), nil
			}
			return "", fmt.Errorf("ExpectRegexReturn(%q): timeout after %s: %w", pattern, timeout, err)
		}
		if re.MatchString(line) {
			return strings.TrimRight(line, "\r\n"), nil
		}
	}
	return "", fmt.Errorf("ExpectRegexReturn(%q): pattern not found within %s", pattern, timeout)
}

// Close closes the underlying TCP connection.
func (c *HeadlessClient) Close() error {
	return c.conn.Close()
}
