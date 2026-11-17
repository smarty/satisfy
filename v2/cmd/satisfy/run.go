package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"

	"github.com/smarty/satisfy/v2/contracts"
)

const (
	exitSuccess       = 0
	exitFailure       = 1
	exitPackageExists = 2
)

// environment carries the process resources a command may use.
type environment struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	logger *slog.Logger
}

// progress returns a progress renderer writing to stderr, or nil when
// progress display is disabled.
func (this environment) progress(enabled bool) contracts.ProgressFunc {
	if !enabled {
		return nil
	}
	return newProgressRenderer(this.stderr).Report
}

// run executes the command line and returns the process exit code.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	env := environment{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		logger: newLogger(stderr),
	}
	return exitCode(env, dispatch(ctx, env, args))
}

func dispatch(ctx context.Context, env environment, args []string) error {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "upload":
		return uploadCommand(ctx, env, args[1:])
	case "check":
		return checkCommand(ctx, env, args[1:])
	case "latest":
		return latestCommand(ctx, env, args[1:])
	case "tags":
		return tagsCommand(ctx, env, args[1:])
	case "version":
		_, err := fmt.Fprintf(env.stdout, "satisfy [%s]\n", softwareVersion())
		return err
	case "download":
		return errors.New("there is no need to supply 'download' as a sub-command")
	default:
		return downloadCommand(ctx, env, args)
	}
}

func exitCode(env environment, err error) int {
	switch {
	case err == nil:
		return exitSuccess
	case errors.Is(err, flag.ErrHelp):
		return exitSuccess
	case errors.Is(err, contracts.ErrPackageExists):
		env.logger.Info(err.Error())
		return exitPackageExists
	default:
		env.logger.Error(err.Error())
		return exitFailure
	}
}

func tagsCommand(ctx context.Context, env environment, args []string) error {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "list":
		return tagsListCommand(ctx, env, args[1:])
	case "modify":
		return tagsModifyCommand(ctx, env, args[1:])
	default:
		return errors.New("usage: satisfy tags <list|modify> [options] (try 'satisfy tags list -h' or 'satisfy tags modify -h')")
	}
}

// newLogger is the CLI's logger: the library is silent by default, and the CLI
// turns logging on by writing text records to stderr.
func newLogger(output io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(output, nil))
}
