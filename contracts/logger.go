package contracts

import (
	"io"

	"github.com/smarty/satisfy/internal/logging"
)

// Logger is the interface used by the CLI to emit structured log output.
type Logger interface {
	// Fatal writes a formatted error message to stderr and exits with code 1.
	//
	// Parameters:
	//   - err: the error to log.
	Fatal(err error)

	// FatalWithLevel writes a formatted error message to stderr with the specified
	// level and exits with code 1.
	//
	// Parameters:
	//   - level: the log severity level.
	//   - err: the error to log.
	FatalWithLevel(level LogLevel, err error)

	// FatalClean writes a formatted error message to stderr without any prefix and
	// exits with code 1.
	//
	// Parameters:
	//   - err: the error to log.
	FatalClean(err error)

	// Log writes a formatted message to stderr with the specified level prefix.
	//
	// Parameters:
	//   - level: the log severity level.
	//   - format: a printf-style format string.
	//   - v: arguments for the format string.
	Log(level LogLevel, format string, v ...any)

	// LogClean writes a formatted message to stderr without any prefix.
	//
	// Parameters:
	//   - format: a printf-style format string.
	//   - v: arguments for the format string.
	LogClean(format string, v ...any)

	// LogLine writes a formatted message to stderr with a trailing newline.
	//
	// Parameters:
	//   - level: the log severity level.
	//   - format: a printf-style format string.
	//   - v: arguments for the format string.
	LogLine(level LogLevel, format string, v ...any)

	// LogLineClean writes a formatted message to stderr without any prefix, with a
	// trailing newline.
	//
	// Parameters:
	//   - format: a printf-style format string.
	//   - v: arguments for the format string.
	LogLineClean(format string, v ...any)

	// Print writes a formatted message to stdout.
	//
	// Parameters:
	//   - level: the log severity level.
	//   - format: a printf-style format string.
	//   - v: arguments for the format string.
	Print(level LogLevel, format string, v ...any)

	// PrintLine writes a formatted message to stdout with a trailing newline.
	//
	// Parameters:
	//   - level: the log severity level.
	//   - format: a printf-style format string.
	//   - v: arguments for the format string.
	PrintLine(level LogLevel, format string, v ...any)

	// PrintClean writes a formatted message to stdout without any prefix.
	//
	// Parameters:
	//   - format: a printf-style format string.
	//   - v: arguments for the format string.
	PrintClean(format string, v ...any)

	// PrintLineClean writes a formatted message to stdout without any prefix, with
	// a trailing newline.
	//
	// Parameters:
	//   - format: a printf-style format string.
	//   - v: arguments for the format string.
	PrintLineClean(format string, v ...any)

	// WriterErr returns the stderr writer used by the logger.
	//
	// Returns:
	//   - io.Writer: the stderr writer.
	WriterErr() io.Writer

	// WriterOut returns the stdout writer used by the logger.
	//
	// Returns:
	//   - io.Writer: the stdout writer.
	WriterOut() io.Writer
}

// NewLogger creates a Logger that writes structured output to the provided
// writers and delegates process exit to exitFunc.
//
// Parameters:
//   - stdOut: the writer for standard output.
//   - stdErr: the writer for standard error.
//   - exitFunc: called with a status code when a fatal log method is invoked.
//
// Returns:
//   - Logger: a configured logger instance.
func NewLogger(stdOut io.Writer, stdErr io.Writer, exitFunc func(int)) Logger {
	return logging.NewLogger(stdOut, stdErr, exitFunc)
}
