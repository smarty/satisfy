package main

import (
	"os"

	"bitbucket.org/smartystreets/satisfy/archive"
)

func main() {
	writer := archive.NewTarArchiveWriter(os.Stdout)
	defer writer.Close()
	writer.WriteHeader("sub/hello.txt", int64(len("Hello, World!")))
	writer.Write([]byte("Hello, World!"))
}
