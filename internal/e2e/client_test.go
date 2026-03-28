package e2e_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/e2e"
)

func startEchoServer(t *testing.T) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 1024)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			_, _ = conn.Write(buf[:n])
		}
	}()
	return ln.Addr().String(), func() { ln.Close() }
}

func startScriptedServer(t *testing.T, responses []string) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		for _, r := range responses {
			fmt.Fprintf(conn, "%s\r\n", r)
		}
	}()
	return ln.Addr().String(), func() { ln.Close() }
}

func TestHeadlessClient_Dial(t *testing.T) {
	addr, stop := startScriptedServer(t, nil)
	defer stop()
	c, err := e2e.Dial(addr)
	require.NoError(t, err)
	require.NotNil(t, c)
	_ = c.Close()
}

func TestHeadlessClient_Dial_BadAddr(t *testing.T) {
	_, err := e2e.Dial("127.0.0.1:1")
	assert.Error(t, err)
}

func TestHeadlessClient_Send(t *testing.T) {
	addr, stop := startEchoServer(t)
	defer stop()
	c, err := e2e.Dial(addr)
	require.NoError(t, err)
	defer c.Close()
	err = c.Send("hello")
	assert.NoError(t, err)
}

func TestHeadlessClient_Expect_Match(t *testing.T) {
	addr, stop := startScriptedServer(t, []string{"Username: "})
	defer stop()
	c, err := e2e.Dial(addr)
	require.NoError(t, err)
	defer c.Close()
	err = c.Expect("Username", 2*time.Second)
	assert.NoError(t, err)
}

func TestHeadlessClient_Expect_Timeout(t *testing.T) {
	addr, stop := startScriptedServer(t, []string{"some other line"})
	defer stop()
	c, err := e2e.Dial(addr)
	require.NoError(t, err)
	defer c.Close()
	err = c.Expect("WILL NEVER APPEAR", 200*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WILL NEVER APPEAR")
}

func TestHeadlessClient_Expect_DefaultTimeout(t *testing.T) {
	addr, stop := startScriptedServer(t, []string{"hello"})
	defer stop()
	c, err := e2e.Dial(addr)
	require.NoError(t, err)
	defer c.Close()
	err = c.Expect("hello", 0)
	assert.NoError(t, err)
}

func TestHeadlessClient_ExpectRegex_Match(t *testing.T) {
	addr, stop := startScriptedServer(t, []string{"Slot 3 set."})
	defer stop()
	c, err := e2e.Dial(addr)
	require.NoError(t, err)
	defer c.Close()
	err = c.ExpectRegex(`Slot \d+ set\.`, 2*time.Second)
	assert.NoError(t, err)
}

func TestHeadlessClient_ExpectRegex_NoMatch(t *testing.T) {
	addr, stop := startScriptedServer(t, []string{"unrelated"})
	defer stop()
	c, err := e2e.Dial(addr)
	require.NoError(t, err)
	defer c.Close()
	err = c.ExpectRegex(`Slot \d+ set\.`, 200*time.Millisecond)
	assert.Error(t, err)
}
