package main

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/smartystreets/gcs"

	"bitbucket.org/smartystreets/satisfy/archive"
	"bitbucket.org/smartystreets/satisfy/build"
	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/fs"
	"bitbucket.org/smartystreets/satisfy/remote"
)

type Config struct {
	sourceDirectory  string
	packageName      string
	packageVersion   string
	remoteBucket     string
	remotePathPrefix string
}

func main() {
	config := Config{}
	flag.StringVar(&config.sourceDirectory, "local", "", "The directory containing package data.")
	flag.StringVar(&config.packageName, "name", "", "The name of the package.")
	flag.StringVar(&config.packageVersion, "version", "", "The version of the package.")
	flag.StringVar(&config.remoteBucket, "remote-bucket", "", "The remote bucket name.")
	flag.StringVar(&config.remotePathPrefix, "remote-prefix", "", "The remote path prefix.")
	flag.Parse()

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
		fs.NewDiskFileSystem(config.sourceDirectory),
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
		Name:    config.packageName,
		Version: config.packageVersion,
		Archive: contracts.Archive{
			Filename:    file.Name(), // TODO: this is wrong
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

	uploader := remote.NewGoogleCloudStorageUploader(http.DefaultClient, credentials, config.remoteBucket)
	// TODO: wrap uploader in retry-uploader

	archiveRequest := contracts.UploadRequest{
		Path:        path.Join(config.remotePathPrefix, "bowling-game_1.2.3.tar.gz"), // TODO: derive
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
		Path:        path.Join(config.remotePathPrefix ,"bowling-game_1.2.3.json"), // TODO: derive
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
