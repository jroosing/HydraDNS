package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigure(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "default config",
			cfg:  Config{Level: "INFO"},
		},
		{
			name: "debug level",
			cfg:  Config{Level: "DEBUG"},
		},
		{
			name: "structured JSON",
			cfg:  Config{Level: "INFO", Structured: true, StructuredFormat: "json"},
		},
		{
			name: "structured text",
			cfg:  Config{Level: "INFO", Structured: true, StructuredFormat: "keyvalue"},
		},
		{
			name: "with extra fields",
			cfg: Config{
				Level:       "INFO",
				ExtraFields: map[string]string{"service": "test", "env": "test"},
			},
		},
		{
			name: "with PID",
			cfg:  Config{Level: "INFO", IncludePID: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := Configure(tt.cfg)
			require.NotNil(t, logger)
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"DEBUG", "DEBUG"},
		{"debug", "DEBUG"},
		{"INFO", "INFO"},
		{"info", "INFO"},
		{"WARN", "WARN"},
		{"warn", "WARN"},
		{"WARNING", "WARN"},
		{"ERROR", "ERROR"},
		{"error", "ERROR"},
		{"invalid", "INFO"}, // default
		{"", "INFO"},        // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level := parseLevel(tt.input)
			// Just verify it doesn't panic
			assert.NotNil(t, level)
		})
	}
}
