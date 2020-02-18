package main

import (
	"fmt"
	"log"
	"os"

	"bitbucket.org/smartystreets/satisfy/core"
	"bitbucket.org/smartystreets/satisfy/shell"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	if isSubCommand("upload") {
		uploadMain(os.Args[2:])
	} else if isSubCommand("check") {
		checkMain(os.Args[2:])
	} else if isSubCommand("version") {
		versionMain()
	} else if isSubCommand("download") {
		log.Fatal("there is no need to supply 'download' as a sub-command")
	} else {
		downloadMain(os.Args[1:])
	}
}

func isSubCommand(name string) bool {
	return len(os.Args) > 1 && os.Args[1] == name
}

func uploadMain(args []string) {
	loader := core.NewUploadConfigLoader(shell.NewDiskFileSystem(""), shell.NewEnvironment(), os.Stdin)
	config, err := loader.LoadConfig("upload", args)
	if err != nil {
		log.Fatal(err)
	}
	NewUploadApp(config).Run()
}

func checkMain(args []string) {
	loader := core.NewUploadConfigLoader(shell.NewDiskFileSystem(""), shell.NewEnvironment(), os.Stdin)
	config, err := loader.LoadConfig("check", args)
	if err != nil {
		log.Fatal(err)
	}
	NewCheckApp(config).Run()
}

func downloadMain(args []string) {
	config, err := parseDownloadConfig(args)
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(NewDownloadApp(config).Run())
}

func versionMain() {
	fmt.Printf("satisfy [%s]\n", ldflagsSoftwareVersion)
}

var ldflagsSoftwareVersion = "debug"
