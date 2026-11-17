package main

import (
	"errors"
	"log"
	"os"
	"runtime/debug"

	"github.com/smarty/satisfy/v2/contracts"
	"github.com/smarty/satisfy/v2/core"
	"github.com/smarty/satisfy/v2/shell"
	"github.com/smarty/satisfy/v2/transfer"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	sub := ""
	if len(os.Args) > 1 {
		sub = os.Args[1]
	}
	switch sub {
	case "upload":
		uploadMain(os.Args[2:])
	case "check":
		checkMain(os.Args[2:])
	case "latest":
		latestMain(os.Args[2:])
	case "tags":
		tagsMain(os.Args[2:])
	case "version":
		versionMain()
	case "download":
		log.Fatal("there is no need to supply 'download' as a sub-command")
	default:
		downloadMain(os.Args[1:])
	}
}

func uploadMain(args []string) {
	loader := core.NewUploadConfigLoader(shell.NewDiskFileSystem(""), shell.NewEnvironment(), os.Stdin, os.Stderr)
	config, err := loader.LoadConfig("upload", args)
	exitOnError(err)
	exitOnError(transfer.NewCheckApp(config).Run())
	exitOnError(transfer.NewUploadApp(config).Run())
}

func checkMain(args []string) {
	loader := core.NewUploadConfigLoader(shell.NewDiskFileSystem(""), shell.NewEnvironment(), os.Stdin, os.Stderr)
	config, err := loader.LoadConfig("check", args)
	exitOnError(err)
	exitOnError(transfer.NewCheckApp(config).Run())
}

func downloadMain(args []string) {
	config, err := transfer.ParseDownloadConfig(args)
	exitOnError(err)
	exitOnError(transfer.NewDownloadApp(config).Run())
}

func tagsMain(args []string) {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "list":
		tagsListMain(args[1:])
	case "modify":
		tagsModifyMain(args[1:])
	default:
		log.Fatal("usage: satisfy tags <list|modify> [options] (try 'satisfy tags list -h' or 'satisfy tags modify -h')")
	}
}

func tagsModifyMain(args []string) {
	loader := core.NewTagsConfigLoader(shell.NewDiskFileSystem(""), shell.NewEnvironment(), os.Stdin, os.Stderr)
	config, err := loader.LoadConfig(args)
	exitOnError(err)
	exitOnError(transfer.NewTagsApp(config).Run())
}

func tagsListMain(args []string) {
	config, err := transfer.ParseTagsListConfig(args)
	exitOnError(err)
	exitOnError(transfer.NewTagsListApp(config).Run())
}

func latestMain(args []string) {
	config, err := transfer.ParseLatestConfig(args)
	exitOnError(err)
	exitOnError(transfer.NewLatestApp(config).Run())
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
