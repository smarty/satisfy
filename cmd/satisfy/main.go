package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	if isSubCommand("upload") {
		uploadMain(os.Args[2:])
	} else if isSubCommand("check") {
		checkMain(os.Args[2:])
	} else if isSubCommand("version") {
		versionMain()
	} else {
		downloadMain(os.Args[1:])
	}
}

func isSubCommand(name string) bool {
	return len(os.Args) > 1 && os.Args[1] == name
}

func uploadMain(args []string) {
	NewUploadApp(parseUploadConfig("upload", args)).Run()
}

func checkMain(args []string) {
	NewCheckApp(parseUploadConfig("check", args)).Run()
}

func downloadMain(args []string) {
	os.Exit(NewDownloadApp(parseDownloadConfig(args)).Run())
}

func versionMain() {
	fmt.Printf("satisfy [%s]\n", ldflagsSoftwareVersion)
}

var ldflagsSoftwareVersion string = "debug"
