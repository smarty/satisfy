package main

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/json"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/smartystreets/gcs"

	"bitbucket.org/smartystreets/satisfy/archive"
	"bitbucket.org/smartystreets/satisfy/build"
	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/fs"
	"bitbucket.org/smartystreets/satisfy/remote"
)

func main() {
	NewApp(parseConfig()).Run()
}

type App struct {
	config     Config
	file       *os.File
	file2      *os.File
	hasher     hash.Hash
	compressor *gzip.Writer
	builder    *build.PackageBuilder
	manifest   contracts.Manifest
	uploader   contracts.Uploader
}

func NewApp(config Config) *App {
	return &App{config: config}
}

func (this *App) Run() {
	var err error
	this.openArchiveFile()

	log.Println("Writing archive at:", this.file.Name())
	this.hasher = md5.New()
	writer := io.MultiWriter(this.hasher, this.file)
	this.compressor = gzip.NewWriter(writer)

	this.builder = build.NewPackageBuilder(
		fs.NewDiskFileSystem(this.config.sourceDirectory),
		archive.NewTarArchiveWriter(this.compressor),
		md5.New(),
	)

	err = this.builder.Build()
	if err != nil {
		log.Fatal(err)
	}

	err = this.compressor.Close()
	if err != nil {
		log.Fatal(err)
	}

	this.closeArchiveFile()

	fileInfo, err := os.Stat(this.file.Name())
	if err != nil {
		log.Fatal(err)
	}

	this.manifest = contracts.Manifest{
		Name:    this.config.packageName,
		Version: this.config.packageVersion,
		Archive: contracts.Archive{
			Filename:    this.file.Name(), // TODO: this is wrong
			Size:        uint64(fileInfo.Size()),
			MD5Checksum: this.hasher.Sum(nil),
			Contents:    this.builder.Contents(),
		},
	}

	this.buildUploader()

	this.openArchiveFile()

	err = this.uploader.Upload(this.buildArchiveUploadRequest())
	if err != nil {
		log.Fatal(err)
	}

	this.closeArchiveFile()

	err = this.uploader.Upload(this.buildManifestUploadRequest())
	if err != nil {
		log.Fatal(err)
	}
}

func (this *App) openArchiveFile() {
	var err error
	if this.file == nil {
		this.file, err = ioutil.TempFile("", "")
	} else {
		this.file, err = os.Open(this.file.Name())
	}
	if err != nil {
		log.Fatal(err)
	}
}
func (this *App) closeArchiveFile(){
	err := this.file.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func (this *App) buildUploader() {
	raw, err := ioutil.ReadFile("gcs-credentials.json") // TODO: ENV?
	if err != nil {
		log.Fatal(err)
	}

	credentials, err := gcs.ParseCredentialsFromJSON(raw)
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{Timeout: time.Minute}
	gcsUploader := remote.NewGoogleCloudStorageUploader(client, credentials, this.config.remoteBucket)
	this.uploader = remote.NewRetryUploader(gcsUploader, 5)
}
func (this *App) buildArchiveUploadRequest() contracts.UploadRequest {
	return contracts.UploadRequest{
		Path:        this.config.composeRemotePath("tar.gz"),
		Body:        NopReadSeekCloser(this.file),
		Size:        int64(this.manifest.Archive.Size),
		ContentType: "application/gzip",
		Checksum:    this.manifest.Archive.MD5Checksum,
	}
}
func (this *App) buildManifestUploadRequest() contracts.UploadRequest {
	buffer := this.writeManifestToBuffer()
	return contracts.UploadRequest{
		Path:        this.config.composeRemotePath("json"),
		Body:        bytes.NewReader(buffer.Bytes()),
		Size:        int64(buffer.Len()),
		ContentType: "application/json",
		Checksum:    this.hasher.Sum(nil),
	}
}
func (this *App) writeManifestToBuffer() *bytes.Buffer {
	this.hasher.Reset()
	buffer := new(bytes.Buffer)
	writer := io.MultiWriter(buffer, this.hasher)
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(this.manifest)
	return buffer
}
