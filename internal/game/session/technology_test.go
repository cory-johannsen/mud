package session_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
)

func TestPreparedSlot_ZeroValue(t *testing.T) {
	var s session.PreparedSlot
	assert.Equal(t, "", s.TechID)
}

func TestInnateSlot_ZeroMaxUses_MeansUnlimited(t *testing.T) {
	s := session.InnateSlot{MaxUses: 0}
	assert.Equal(t, 0, s.MaxUses) // 0 = unlimited per spec
}

func TestPlayerSession_TechFields_NilUntilLoaded(t *testing.T) {
	sess := &session.PlayerSession{}
	assert.Nil(t, sess.HardwiredTechs)
	assert.Nil(t, sess.PreparedTechs)
	assert.Nil(t, sess.KnownTechs)
	assert.Nil(t, sess.InnateTechs)
}
