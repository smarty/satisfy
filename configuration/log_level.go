package configuration

import "github.com/smarty/satisfy/internal/logging"

const (
	// Info is the default value.
	Info = LogLevel(logging.Info)

	// No prefix means no date-time, file, or line-number prefix.
	NoPrefix = LogLevel(logging.NoPrefix)
	Warning  = LogLevel(logging.Warning)
	Error    = LogLevel(logging.Error)
)

// LogLevel represents a log severity level for categorizing log messages.
type LogLevel = logging.Level
