package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Level            string
	Structured       bool
	StructuredFormat string
	IncludePID       bool
	ExtraFields      map[string]string
}

func Configure(cfg Config) *slog.Logger {
	level := parseLevel(cfg.Level)
	var handler slog.Handler
	out := io.Writer(os.Stderr)

	attrs := make([]slog.Attr, 0, len(cfg.ExtraFields)+1)
	for k, v := range cfg.ExtraFields {
		attrs = append(attrs, slog.String(k, v))
	}
	if cfg.IncludePID {
		attrs = append(attrs, slog.Int("pid", os.Getpid()))
	}

	if cfg.Structured {
		if strings.ToLower(cfg.StructuredFormat) == "json" {
			handler = slog.NewJSONHandler(out, &slog.HandlerOptions{Level: level})
		} else {
			// key=value-ish output
			handler = slog.NewTextHandler(out, &slog.HandlerOptions{Level: level})
		}
	} else {
		handler = slog.NewTextHandler(out, &slog.HandlerOptions{Level: level})
	}

	if len(attrs) > 0 {
		handler = handler.WithAttrs(attrs)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

func parseLevel(s string) slog.Level {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
