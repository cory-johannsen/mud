package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
)

func TestWelcomeBannerContainsKeyElements(t *testing.T) {
	// Verify the banner contains expected elements
	stripped := telnet.StripANSI(welcomeBanner)
	assert.Contains(t, stripped, "GUNCHETE")
	assert.Contains(t, stripped, "login")
	assert.Contains(t, stripped, "register")
	assert.Contains(t, stripped, "quit")
}
