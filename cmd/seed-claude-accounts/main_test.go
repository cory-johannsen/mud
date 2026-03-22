package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestClaudeAccounts_HasThreeEntries(t *testing.T) {
	assert.Len(t, claudeAccounts, 3)
}

func TestClaudeAccounts_Roles(t *testing.T) {
	roles := map[string]bool{}
	for _, a := range claudeAccounts {
		roles[a.role] = true
	}
	assert.True(t, roles["player"], "must have player role")
	assert.True(t, roles["editor"], "must have editor role")
	assert.True(t, roles["admin"], "must have admin role")
}

func TestClaudeAccounts_Usernames(t *testing.T) {
	for _, a := range claudeAccounts {
		assert.NotEmpty(t, a.username)
		assert.Contains(t, a.username, "claude_")
	}
}

func TestClaudeAccounts_UsernamesProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// All entries must have non-empty usernames and valid roles.
		for _, a := range claudeAccounts {
			assert.NotEmpty(rt, a.username)
			assert.NotEmpty(rt, a.role)
		}
	})
}
