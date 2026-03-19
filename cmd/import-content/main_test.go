package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLI_LocalizeWithoutKey_ReturnsError(t *testing.T) {
	os.Unsetenv("ANTHROPIC_API_KEY")
	err := run([]string{"-format", "pf2e", "-source", t.TempDir(), "-output", t.TempDir(), "-localize"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key")
}

func TestCLI_UnknownFormat_ReturnsError(t *testing.T) {
	err := run([]string{"-format", "unknown", "-source", t.TempDir(), "-output", t.TempDir()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown format")
}

func TestCLI_MissingFormat_ReturnsError(t *testing.T) {
	err := run([]string{"-source", t.TempDir(), "-output", t.TempDir()})
	require.Error(t, err)
}

func TestCLI_MissingSource_ReturnsError(t *testing.T) {
	err := run([]string{"-format", "pf2e", "-output", t.TempDir()})
	require.Error(t, err)
}

func TestCLI_PF2E_EmptySourceDir_NoError(t *testing.T) {
	err := run([]string{"-format", "pf2e", "-source", t.TempDir(), "-output", t.TempDir()})
	require.NoError(t, err)
}
