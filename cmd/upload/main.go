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
	"path/filepath"
	"time"

	"github.com/klauspost/compress/zstd"

	"bitbucket.org/smartystreets/satisfy/archive"
	"bitbucket.org/smartystreets/satisfy/cmd"
	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/core"
	"bitbucket.org/smartystreets/satisfy/fs"
	"bitbucket.org/smartystreets/satisfy/remote"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	NewApp(cmd.ParseConfig()).Run()
	log.Println("OK")
}

type App struct {
	config     cmd.Config
	file       *os.File
	hasher     hash.Hash
	compressor io.WriteCloser
	builder    *core.PackageBuilder
	manifest   contracts.Manifest
	client     contracts.RemoteStorage
}

func NewApp(config cmd.Config) *App {
	return &App{config: config}
}

func (this *App) Run() {
	this.buildRemoteStorageClient()

	log.Println("Building the archive...")
	this.buildArchiveAndManifestContents()
	this.completeManifest()

	log.Println("Manifest:", this.dumpManifest())

	log.Println("Uploading the archive...")
	this.upload(this.buildArchiveUploadRequest())
	this.closeArchiveFile()
	this.deleteLocalArchiveFile()

	log.Println("Uploading the manifest...")
	this.upload(this.buildManifestUploadRequest())
}

func (this *App) buildArchiveUploadRequest() contracts.UploadRequest {
	this.openArchiveFile()
	return contracts.UploadRequest{
		Bucket:      this.config.RemoteBucket,
		Resource:    this.config.ComposeRemotePath(cmd.RemoteArchiveFilename),
		Body:        NewFileWrapper(this.file),
		Size:        int64(this.manifest.Archive.Size),
		ContentType: "application/zstd",
		Checksum:    this.manifest.Archive.MD5Checksum,
	}
}

func (this *App) buildArchiveAndManifestContents() {
	var err error
	this.file, err = ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}
	this.hasher = md5.New()
	writer := io.MultiWriter(this.hasher, this.file)
	this.InitializeCompressor(writer)

	this.builder = core.NewPackageBuilder(
		fs.NewDiskFileSystem(this.config.SourceDirectory),
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
}

func (this *App) InitializeCompressor(writer io.Writer) {
	factory, found := compression[this.config.CompressionAlgorithm]
	if !found {
		log.Fatalln("Unsupported compression algorithm:", this.config.CompressionAlgorithm)
	}
	this.compressor = factory(writer, this.config.CompressionLevel)
}

var compression = map[string]func(_ io.Writer, level int) io.WriteCloser{
	"zstd": func(writer io.Writer, level int) io.WriteCloser {
		compressor, err := zstd.NewWriter(writer, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)))
		if err != nil {
			log.Fatal(err)
		}
		return compressor
	},
	"gzip": func(writer io.Writer, level int) io.WriteCloser {
		compressor, err := gzip.NewWriterLevel(writer, level)
		if err != nil {
			log.Panicln(err)
		}
		return compressor
	},
}

func (this *App) buildManifestUploadRequest() contracts.UploadRequest {
	buffer := this.writeManifestToBuffer()
	return contracts.UploadRequest{
		Bucket:      this.config.RemoteBucket,
		Resource:    this.config.ComposeRemotePath(cmd.RemoteManifestFilename),
		Body:        bytes.NewReader(buffer.Bytes()),
		Size:        int64(buffer.Len()),
		ContentType: "application/json",
		Checksum:    this.hasher.Sum(nil),
	}
}

func (this *App) buildRemoteStorageClient() {
	client := &http.Client{Timeout: time.Minute} // TODO: clean http.Client and Transport
	gcsClient := remote.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, http.StatusOK)
	this.client = remote.NewRetryClient(gcsClient, this.config.MaxRetry)
}

func (this *App) completeManifest() {
	fileInfo, err := os.Stat(this.file.Name())
	if err != nil {
		log.Fatal(err)
	}
	this.manifest = contracts.Manifest{
		Name:    this.config.PackageName,
		Version: this.config.PackageVersion,
		Archive: contracts.Archive{
			Filename:             filepath.Base(this.config.ComposeRemotePath(cmd.RemoteArchiveFilename)),
			Size:                 uint64(fileInfo.Size()),
			MD5Checksum:          this.hasher.Sum(nil),
			Contents:             this.builder.Contents(),
			CompressionAlgorithm: this.config.CompressionAlgorithm,
		},
	}
}

func (this *App) closeArchiveFile() {
	err := this.file.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func (this *App) deleteLocalArchiveFile() {
	err := os.Remove(this.file.Name())
	if err != nil {
		log.Fatal(err)
	}
}

func (this *App) openArchiveFile() {
	var err error
	this.file, err = os.Open(this.file.Name())
	if err != nil {
		log.Fatal(err)
	}
}

func (this *App) upload(request contracts.UploadRequest) {
	err := this.client.Upload(request)
	if err != nil {
		log.Fatal(err)
	}
}

func (this *App) writeManifestToBuffer() *bytes.Buffer {
	buffer := new(bytes.Buffer)
	this.hasher.Reset()
	writer := io.MultiWriter(buffer, this.hasher)
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(this.manifest)
	return buffer
}

func (this *App) dumpManifest() string {
	raw, err := json.MarshalIndent(this.manifest, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	return string(raw)
}
