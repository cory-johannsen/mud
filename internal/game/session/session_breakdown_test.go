package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlayerSession_ShowDamageBreakdown_DefaultFalse(t *testing.T) {
	sess := &PlayerSession{}
	assert.False(t, sess.ShowDamageBreakdown)
}
