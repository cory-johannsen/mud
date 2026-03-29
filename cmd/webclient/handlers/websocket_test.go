package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/cmd/webclient/handlers"
)

func TestWSHandler_RejectsInvalidJWT(t *testing.T) {
	h := handlers.NewWSHandler("test-secret", nil, nil)
	srv := httptest.NewServer(http.HandlerFunc(h.ServeHTTP))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "?token=not-a-valid-jwt"
	_, resp, err := websocket.DefaultDialer.Dial(url, nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWSHandler_RejectsMissingToken(t *testing.T) {
	h := handlers.NewWSHandler("test-secret", nil, nil)
	srv := httptest.NewServer(http.HandlerFunc(h.ServeHTTP))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	_, resp, err := websocket.DefaultDialer.Dial(url, nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
