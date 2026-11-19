package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// The first signal cancels ctx. Stop catching signals then, so that a
	// second one ends the process at once.
	context.AfterFunc(ctx, stop)
	code := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// softwareVersion prefers the version set at build time with -ldflags, falling back to the
// module version Go records in the binary
func softwareVersion() string {
	if ldflagsSoftwareVersion != "" && ldflagsSoftwareVersion != "debug" {
		return ldflagsSoftwareVersion
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "debug"
}

var ldflagsSoftwareVersion = "debug"
