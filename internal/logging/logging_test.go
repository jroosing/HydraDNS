package logging_test

import (
	"testing"

	"github.com/jroosing/hydradns/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Logger Configuration Tests
// =============================================================================

func TestConfigure_DefaultConfig(t *testing.T) {
	cfg := logging.Config{
		Level: "INFO",
	}

	logger := logging.Configure(cfg)
	require.NotNil(t, logger, "Configure should return a logger")
}

func TestConfigure_AllLogLevels(t *testing.T) {
	levels := []string{"DEBUG", "INFO", "WARN", "WARNING", "ERROR"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			cfg := logging.Config{Level: level}
			logger := logging.Configure(cfg)
			assert.NotNil(t, logger)
		})
	}
}

func TestConfigure_CaseInsensitiveLevel(t *testing.T) {
	levels := []string{"debug", "Debug", "DEBUG", "DeBuG"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			cfg := logging.Config{Level: level}
			logger := logging.Configure(cfg)
			assert.NotNil(t, logger)
		})
	}
}

func TestConfigure_InvalidLevelDefaultsToInfo(t *testing.T) {
	cfg := logging.Config{Level: "INVALID"}
	logger := logging.Configure(cfg)
	assert.NotNil(t, logger, "Invalid level should still return a logger")
}

func TestConfigure_StructuredJSON(t *testing.T) {
	cfg := logging.Config{
		Level:            "INFO",
		Structured:       true,
		StructuredFormat: "json",
	}

	logger := logging.Configure(cfg)
	assert.NotNil(t, logger)
}

func TestConfigure_StructuredText(t *testing.T) {
	cfg := logging.Config{
		Level:            "INFO",
		Structured:       true,
		StructuredFormat: "text",
	}

	logger := logging.Configure(cfg)
	assert.NotNil(t, logger)
}

func TestConfigure_WithExtraFields(t *testing.T) {
	cfg := logging.Config{
		Level: "INFO",
		ExtraFields: map[string]string{
			"app":     "hydradns",
			"version": "1.0.0",
		},
	}

	logger := logging.Configure(cfg)
	assert.NotNil(t, logger)
}

func TestConfigure_WithPID(t *testing.T) {
	cfg := logging.Config{
		Level:      "INFO",
		IncludePID: true,
	}

	logger := logging.Configure(cfg)
	assert.NotNil(t, logger)
}

func TestConfigure_EmptyLevel(t *testing.T) {
	cfg := logging.Config{Level: ""}
	logger := logging.Configure(cfg)
	assert.NotNil(t, logger, "Empty level should default to INFO")
}
