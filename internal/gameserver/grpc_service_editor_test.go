package gameserver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/gameserver"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestRequireEditor_AllowsEditorRole(t *testing.T) {
	sess := &session.PlayerSession{Role: postgres.RoleEditor}
	assert.Nil(t, gameserver.RequireEditor(sess))
}

func TestRequireEditor_AllowsAdminRole(t *testing.T) {
	sess := &session.PlayerSession{Role: postgres.RoleAdmin}
	assert.Nil(t, gameserver.RequireEditor(sess))
}

func TestRequireEditor_DeniesPlayerRole(t *testing.T) {
	sess := &session.PlayerSession{Role: postgres.RolePlayer}
	evt := gameserver.RequireEditor(sess)
	assert.NotNil(t, evt)
}

func TestRequireAdmin_AllowsAdminRole(t *testing.T) {
	sess := &session.PlayerSession{Role: postgres.RoleAdmin}
	assert.Nil(t, gameserver.RequireAdmin(sess))
}

func TestRequireAdmin_DeniesEditorRole(t *testing.T) {
	sess := &session.PlayerSession{Role: postgres.RoleEditor}
	evt := gameserver.RequireAdmin(sess)
	assert.NotNil(t, evt)
}

// Property: requireEditor denies all roles except editor and admin.
func TestRequireEditorProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		role := rapid.StringOf(rapid.Rune()).Draw(t, "role")
		sess := &session.PlayerSession{Role: role}
		evt := gameserver.RequireEditor(sess)
		if role == postgres.RoleEditor || role == postgres.RoleAdmin {
			assert.Nil(t, evt)
		} else {
			assert.NotNil(t, evt)
		}
	})
}
