package main

import (
	"crypto/md5"
	"log"
	"os"

	"bitbucket.org/smartystreets/satisfy/archive"
	"bitbucket.org/smartystreets/satisfy/build"
	"bitbucket.org/smartystreets/satisfy/fs"
)

func main() {
	builder := build.NewPackageBuilder(fs.NewDiskFileSystem("/Users/Gordon/Desktop/bowling"),archive.NewTarArchiveWriter(os.Stdout),md5.New())

	err := builder.Build()
	if err != nil {
		log.Fatal(err)
	}

	log.Println(builder.Contents())
}
