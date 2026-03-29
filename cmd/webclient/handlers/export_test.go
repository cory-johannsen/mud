package handlers

import (
	"encoding/json"

	"github.com/cory-johannsen/mud/internal/game/command"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// WSMessageForTest constructs a wsMessage for use in tests.
func WSMessageForTest(msgType string, payload any) wsMessage {
	raw, _ := json.Marshal(payload)
	return wsMessage{Type: msgType, Payload: raw}
}

// DispatchWSMessageForTest exposes dispatchWSMessage for unit tests.
func DispatchWSMessageForTest(env wsMessage, reqID string, registry *command.Registry) (*gamev1.ClientMessage, error) {
	return dispatchWSMessage(env, reqID, registry)
}
