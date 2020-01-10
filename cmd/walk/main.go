package main

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/smartystreets/gcs"

	"bitbucket.org/smartystreets/satisfy/archive"
	"bitbucket.org/smartystreets/satisfy/build"
	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/fs"
	"bitbucket.org/smartystreets/satisfy/remote"
)

func main() {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}
	localPath := file.Name()
	log.Println(localPath)
	hasher := md5.New()
	writer := io.MultiWriter(hasher, file)
	compressor := gzip.NewWriter(writer)

	builder := build.NewPackageBuilder(
		fs.NewDiskFileSystem("/Users/Mike/src/github.com/smartystreets/gunit/advanced_examples"), // TODO: CLI
		archive.NewTarArchiveWriter(writer),
		md5.New(),
	)

	err = builder.Build()
	if err != nil {
		log.Fatal(err)
	}

	err = compressor.Close()
	if err != nil {
		log.Fatal(err)
	}

	err = file.Close()
	if err != nil {
		log.Fatal(err)
	}

	fileInfo, err := os.Stat(localPath)
	if err != nil {
		log.Fatal(err)
	}

	manifest := contracts.Manifest{
		Name:    "bowling-game",
		Version: "1.2.3",
		Archive: contracts.Archive{
			Filename:    file.Name(),
			Size:        uint64(fileInfo.Size()),
			MD5Checksum: hasher.Sum(nil),
			Contents:    builder.Contents(),
		},
	}


	raw, err := ioutil.ReadFile("gcs-credentials.json") // TODO: ENV?
	if err != nil {
		log.Fatal(err)
	}

	credentials, err := gcs.ParseCredentialsFromJSON(raw)
	if err != nil {
		log.Fatal(err)
	}

	body, err := os.Open(localPath)
	if err != nil {
		log.Fatal(err)
	}
	defer body.Close()

	uploader := remote.NewGoogleCloudStorageUploader(http.DefaultClient, credentials, "api-gateway-whitelist-downloader") // TODO: CLI
	// TODO: wrap uploader in retry-uploader

	archiveRequest := contracts.UploadRequest{
		Path:        "bowling-game/bowling-game_1.2.3.tar.gz", // TODO: derive
		Body:        body,
		Size:        int64(manifest.Archive.Size),
		ContentType: "application/gzip",
		Checksum:    manifest.Archive.MD5Checksum,
	}
	err = uploader.Upload(archiveRequest)
	if err != nil {
		log.Fatal(err)
	}

	buffer := new(bytes.Buffer)
	hasher.Reset()
	writer = io.MultiWriter(buffer, hasher)
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(manifest)
	manifestRequest := contracts.UploadRequest{
		Path:        "bowling-game/bowling-game_1.2.3.json", // TODO: derive
		Body:        bytes.NewReader(buffer.Bytes()),
		Size:        int64(buffer.Len()),
		ContentType: "application/json",
		Checksum:    hasher.Sum(nil),
	}
	err = uploader.Upload(manifestRequest)
	if err != nil {
		log.Fatal(err)
	}

}
