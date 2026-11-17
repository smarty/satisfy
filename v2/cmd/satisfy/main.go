package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/smarty/satisfy/v2/contracts"
	"github.com/smarty/satisfy/v2/core"
	"github.com/smarty/satisfy/v2/shell"
	"github.com/smarty/satisfy/v2/transfer"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sub := ""
	if len(os.Args) > 1 {
		sub = os.Args[1]
	}
	switch sub {
	case "upload":
		uploadMain(ctx, os.Args[2:])
	case "check":
		checkMain(ctx, os.Args[2:])
	case "latest":
		latestMain(ctx, os.Args[2:])
	case "tags":
		tagsMain(ctx, os.Args[2:])
	case "version":
		versionMain()
	case "download":
		log.Fatal("there is no need to supply 'download' as a sub-command")
	default:
		downloadMain(ctx, os.Args[1:])
	}
}

func uploadMain(ctx context.Context, args []string) {
	loader := core.NewUploadConfigLoader(shell.NewDiskFileSystem(""), shell.NewEnvironment(), os.Stdin, os.Stderr)
	config, err := loader.LoadConfig(ctx, "upload", args)
	exitOnError(err)
	exitOnError(transfer.NewCheckApp(config).Run(ctx))
	exitOnError(transfer.NewUploadApp(config).Run(ctx))
}

func checkMain(ctx context.Context, args []string) {
	loader := core.NewUploadConfigLoader(shell.NewDiskFileSystem(""), shell.NewEnvironment(), os.Stdin, os.Stderr)
	config, err := loader.LoadConfig(ctx, "check", args)
	exitOnError(err)
	exitOnError(transfer.NewCheckApp(config).Run(ctx))
}

func downloadMain(ctx context.Context, args []string) {
	config, err := transfer.ParseDownloadConfig(args)
	exitOnError(err)
	exitOnError(transfer.NewDownloadApp(config).Run(ctx))
}

func tagsMain(ctx context.Context, args []string) {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "list":
		tagsListMain(ctx, args[1:])
	case "modify":
		tagsModifyMain(ctx, args[1:])
	default:
		log.Fatal("usage: satisfy tags <list|modify> [options] (try 'satisfy tags list -h' or 'satisfy tags modify -h')")
	}
}

func tagsModifyMain(ctx context.Context, args []string) {
	loader := core.NewTagsConfigLoader(shell.NewDiskFileSystem(""), shell.NewEnvironment(), os.Stdin, os.Stderr)
	config, err := loader.LoadConfig(ctx, args)
	exitOnError(err)
	exitOnError(transfer.NewTagsApp(config).Run(ctx))
}

func tagsListMain(ctx context.Context, args []string) {
	config, err := transfer.ParseTagsListConfig(args)
	exitOnError(err)
	exitOnError(transfer.NewTagsListApp(config).Run(ctx))
}

func latestMain(ctx context.Context, args []string) {
	config, err := transfer.ParseLatestConfig(args)
	exitOnError(err)
	exitOnError(transfer.NewLatestApp(config).Run(ctx))
}

// exitOnError terminates the process for a non-nil error, using exit code 2
// when the package being uploaded already exists and 1 otherwise.
func exitOnError(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, contracts.ErrPackageExists) {
		log.Println("[INFO]", err)
		os.Exit(2)
	}
	log.Fatal(err)
}

func versionMain() {
	log.Printf("satisfy [%s]\n", softwareVersion())
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
