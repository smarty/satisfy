package main

import (
	"log"
	"os"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	if isSubCommand("upload") {
		uploadMain(os.Args[2:])
	} else if isSubCommand("check") {
		checkMain(os.Args[2:])
	} else {
		downloadMain(os.Args[1:])
	}
}

func isSubCommand(name string) bool {
	return len(os.Args) > 1 && os.Args[1] == name
}
