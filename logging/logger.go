package logging

import (
	"fmt"
	"io"
	"runtime"
	"time"
)

// Logger provides structured logging to stdout and stderr.
type Logger struct {
	stdErr   io.Writer
	stdOut   io.Writer
	exitFunc func(int)
}

// NewLogger creates a new [Logger] instance with the specified output writers.
//
// Parameters:
//   - stdOut: the writer for standard output (used by [Logger.Print] methods).
//   - stdErr: the writer for standard error (used by [Logger.Log] methods).
//   - exitFunc: a function to call when exiting the program (used by
//     [Logger.Fatal]).
//
// Returns:
//   - *Logger: a configured logger instance.
func NewLogger(stdOut io.Writer, stdErr io.Writer, exitFunc func(int)) *Logger {
	return &Logger{
		stdErr:   stdErr,
		stdOut:   stdOut,
		exitFunc: exitFunc,
	}
}

// Fatal writes a formatted error message to stderr and exits with code 1.
//
// Parameters:
//   - err: the error to log.
func (this Logger) Fatal(err error) {
	printf(this.stdErr, Error, "%v\n", err)
	this.exitFunc(1)
}

// Log writes a formatted message to stderr with the specified level prefix.
//
// Parameters:
//   - level: the log severity level.
//   - format: a printf-style format string.
//   - v: arguments for the format string.
func (this Logger) Log(level Level, format string, v ...any) {
	printf(this.stdErr, level, format, v...)
}

// LogClean writes a formatted message to stderr without any prefix.
//
// Parameters:
//   - format: a printf-style format string.
//   - v: arguments for the format string.
func (this Logger) LogClean(format string, v ...any) {
	printfSimple(this.stdErr, format, v...)
}

// LogLine writes a formatted message to stderr with a trailing newline.
//
// Parameters:
//   - level: the log severity level.
//   - format: a printf-style format string.
//   - v: arguments for the format string.
func (this Logger) LogLine(level Level, format string, v ...any) {
	printf(this.stdErr, level, format+"\n", v...)
}

// LogLineClean writes a formatted message to stderr without any prefix, with a
// trailing newline.
//
// Parameters:
//   - format: a printf-style format string.
//   - v: arguments for the format string.
func (this Logger) LogLineClean(format string, v ...any) {
	printfSimple(this.stdErr, format+"\n", v...)
}

// Print writes a formatted message to stdout.
//
// Parameters:
//   - level: the log severity level.
//   - format: a printf-style format string.
//   - v: arguments for the format string.
func (this Logger) Print(level Level, format string, v ...any) {
	printf(this.stdOut, level, format, v...)
}

// PrintLine writes a formatted message to stdout with a trailing newline.
//
// Parameters:
//   - level: the log severity level.
//   - format: a printf-style format string.
//   - v: arguments for the format string.
func (this Logger) PrintLine(level Level, format string, v ...any) {
	printf(this.stdOut, level, format+"\n", v...)
}

// PrintClean writes a formatted message to stdout without any prefix.
//
// Parameters:
//   - format: a printf-style format string.
//   - v: arguments for the format string.
func (this Logger) PrintClean(format string, v ...any) {
	printfSimple(this.stdOut, format, v...)
}

// PrintLineClean writes a formatted message to stdout without any prefix, with
// a trailing newline.
//
// Parameters:
//   - format: a printf-style format string.
//   - v: arguments for the format string.
func (this Logger) PrintLineClean(format string, v ...any) {
	printfSimple(this.stdOut, format+"\n", v...)
}

func printf(writer io.Writer, level Level, format string, v ...any) {
	if level == NoPrefix {
		printfSimple(writer, format, v...)
		return
	}

	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???" // same behavior as log.Lshortfile
		line = 0
	}

	file = shortenFilePath(file)
	prefix := fmt.Sprintf("%s %s:%d: %s", time.Now().Format("2006-01-02 15:04:05"), file, line, level.String())
	printfSimple(writer, "%s "+format+"", append([]any{prefix}, v...)...)
}

func printfSimple(writer io.Writer, format string, v ...any) {
	_, _ = fmt.Fprintf(writer, format, v...)
}

func shortenFilePath(file string) string {
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			file = file[i+1:]
			break
		}
	}

	return file
}
