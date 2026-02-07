// Package logging provides structured logging for wacli.
// Log level is controlled via WACLI_LOG environment variable.
// Values: trace, debug, info, warn, error (default: warn)
package logging

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

func init() {
	// Configure zerolog for console output to stderr
	output := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
		NoColor:    os.Getenv("NO_COLOR") != "",
	}

	// Set log level from environment
	level := parseLevel(os.Getenv("WACLI_LOG"))
	zerolog.SetGlobalLevel(level)

	log = zerolog.New(output).With().Timestamp().Logger()
}

func parseLevel(s string) zerolog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "":
		// Default: only show warnings and errors (silent by default)
		return zerolog.WarnLevel
	default:
		return zerolog.InfoLevel
	}
}

// Get returns the global logger.
func Get() zerolog.Logger {
	return log
}

// Trace logs at trace level.
func Trace() *zerolog.Event {
	return log.Trace()
}

// Debug logs at debug level.
func Debug() *zerolog.Event {
	return log.Debug()
}

// Info logs at info level.
func Info() *zerolog.Event {
	return log.Info()
}

// Warn logs at warn level.
func Warn() *zerolog.Event {
	return log.Warn()
}

// Error logs at error level.
func Error() *zerolog.Event {
	return log.Error()
}

// WithComponent returns a logger with component field.
func WithComponent(component string) zerolog.Logger {
	return log.With().Str("component", component).Logger()
}
