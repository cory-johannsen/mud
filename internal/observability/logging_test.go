package observability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/config"
)

func TestNewLogger_JSON(t *testing.T) {
	cfg := config.LoggingConfig{Level: "info", Format: "json"}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestNewLogger_Console(t *testing.T) {
	cfg := config.LoggingConfig{Level: "debug", Format: "console"}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestNewLogger_InvalidLevel(t *testing.T) {
	cfg := config.LoggingConfig{Level: "trace", Format: "json"}
	_, err := NewLogger(cfg)
	assert.Error(t, err)
}

func TestNewLogger_InvalidFormat(t *testing.T) {
	cfg := config.LoggingConfig{Level: "info", Format: "xml"}
	_, err := NewLogger(cfg)
	assert.Error(t, err)
}

func TestNewLogger_AllLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		cfg := config.LoggingConfig{Level: level, Format: "json"}
		logger, err := NewLogger(cfg)
		require.NoError(t, err, "level %q should be valid", level)
		assert.NotNil(t, logger)
	}
}
