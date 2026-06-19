package logger

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var Log zerolog.Logger

// Initialize sets up the logger based on configuration
// debug: enables pretty console output if true
// logLevel: sets the global log level (trace, debug, info, warn, error, fatal, panic)
func Initialize(debug bool, logLevel string) {
	level := parseLogLevel(logLevel)
	if level == zerolog.InfoLevel && strings.ToLower(logLevel) != "info" {
		Log.Warn().Str("logLevel", logLevel).Msg("Invalid log level, defaulting to 'info'")
	}
	zerolog.SetGlobalLevel(level)

	if debug {
		Log.Debug().Msg("Logger using pretty console output (development mode)")
		Log = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		})
	} else {
		Log.Debug().Msg("Logger using JSON output (production mode)")
		Log = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	Log.Info().
		Str("level", level.String()).
		Bool("debug", debug).
		Msg("Logger initialized")
}

func parseLogLevel(logLevel string) zerolog.Level {
	switch strings.ToLower(logLevel) {
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
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}

func DisplayBanner() {
	banner := `
╔═══════════════════════════════════════╗
║            WebUI Skeleton             ║
║                                       ║
║    A Go web application skeleton      ║
║    with authentication & web UI       ║
╚═══════════════════════════════════════╝
`
	Log.Info().Msg(banner)
}
