package main

import (
	"log"
	"os"

	"github.com/smarty/satisfy/core"
	"github.com/smarty/satisfy/shell"
	"github.com/smarty/satisfy/transfer"
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
	if err != nil {
		log.Fatal(err)
	}
	transfer.NewUploadApp(config).Run()
}

func checkMain(args []string) {
	loader := core.NewUploadConfigLoader(shell.NewDiskFileSystem(""), shell.NewEnvironment(), os.Stdin, os.Stderr)
	config, err := loader.LoadConfig("check", args)
	if err != nil {
		log.Fatalln(err)
	}
	transfer.NewCheckApp(config).Run()
}

func downloadMain(args []string) {
	config, err := transfer.ParseDownloadConfig(args)
	if err != nil {
		log.Fatal(err)
	}
	transfer.NewDownloadApp(config).Run()
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
	if err != nil {
		log.Fatal(err)
	}
	transfer.NewTagsApp(config).Run()
}

func tagsListMain(args []string) {
	config, err := transfer.ParseTagsListConfig(args)
	if err != nil {
		log.Fatal(err)
	}
	transfer.NewTagsListApp(config).Run()
}

func latestMain(args []string) {
	config, err := transfer.ParseLatestConfig(args)
	if err != nil {
		log.Fatal(err)
	}
	transfer.NewLatestApp(config).Run()
}

func versionMain() {
	log.Printf("satisfy [%s]\n", ldflagsSoftwareVersion)
}

var ldflagsSoftwareVersion = "debug"
