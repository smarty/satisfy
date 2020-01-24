package main

import (
	"log"
	"os"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	if len(os.Args) == 1 {
		os.Args = append(os.Args, "")
	}
	switch os.Args[1] {
	case "upload":
		uploadMain(os.Args[2:])
	case "check":
		checkMain(os.Args[2:])
	default:
		downloadMain()
	}
}
