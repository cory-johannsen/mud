// internal/frontend/handlers/input_mode_test.go
package handlers_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/frontend/handlers"
	"github.com/stretchr/testify/assert"
)

func TestInputMode_String(t *testing.T) {
	cases := []struct {
		mode handlers.InputMode
		want string
	}{
		{handlers.ModeRoom, "room"},
		{handlers.ModeMap, "map"},
		{handlers.ModeInventory, "inventory"},
		{handlers.ModeCharSheet, "charsheet"},
		{handlers.ModeEditor, "editor"},
		{handlers.ModeCombat, "combat"},
		{handlers.InputMode(99), "unknown"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.mode.String())
		})
	}
}
