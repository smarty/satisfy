package transfer

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
	"net/url"
	"os"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/smartystreets/satisfy/contracts"
	"github.com/smartystreets/satisfy/core"
	"github.com/smartystreets/satisfy/shell"
)

type UploadApp struct {
	config        contracts.UploadConfig
	packageConfig contracts.PackageConfig
	file          *os.File
	hasher        hash.Hash
	compressor    io.WriteCloser
	builder       *core.PackageBuilder
	manifest      contracts.Manifest
	client        contracts.RemoteStorage
}

func NewUploadApp(config contracts.UploadConfig) *UploadApp {
	NewCheckApp(config).Run()
	return &UploadApp{config: config, packageConfig: config.PackageConfig}
}

func (this *UploadApp) Run() {
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
	this.upload(this.buildManifestUploadRequest(this.packageConfig.ComposeRemoteAddress(contracts.RemoteManifestFilename)))
	this.upload(this.buildManifestUploadRequest(this.packageConfig.ComposeLatestManifestRemoteAddress()))
}

func (this *UploadApp) buildArchiveUploadRequest() contracts.UploadRequest {
	this.openArchiveFile()
	return contracts.UploadRequest{
		RemoteAddress: this.packageConfig.ComposeRemoteAddress(contracts.RemoteArchiveFilename),
		Body:          NewFileWrapper(this.file),
		Size:          int64(this.manifest.Archive.Size),
		ContentType:   contentType[this.manifest.Archive.CompressionAlgorithm],
		Checksum:      this.manifest.Archive.MD5Checksum,
	}
}

func (this *UploadApp) buildArchiveAndManifestContents() {
	var err error
	this.file, err = ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}
	this.hasher = md5.New()
	writer := io.MultiWriter(this.hasher, this.file)
	this.InitializeCompressor(writer)

	this.builder = core.NewPackageBuilder(
		shell.NewDiskFileSystem(this.packageConfig.SourceDirectory),
		shell.NewSwitchArchiveWriter(this.compressor),
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

func (this *UploadApp) InitializeCompressor(writer io.Writer) {
	factory, found := compression[this.packageConfig.CompressionAlgorithm]
	if !found {
		log.Fatalln("Unsupported compression algorithm:", this.packageConfig.CompressionAlgorithm)
	}
	this.compressor = factory(writer, this.packageConfig.CompressionLevel)
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
	"zip": func(writer io.Writer, level int) io.WriteCloser {
		return shell.NewZipArchiveWriter(writer, level)
	},
}
var contentType = map[string]string{
	"zstd": "application/zstd",
	"gzip": "application/gzip",
	"zip":  "application/zip",
}

func (this *UploadApp) buildManifestUploadRequest(remoteAddress url.URL) contracts.UploadRequest {
	buffer := this.writeManifestToBuffer()
	return contracts.UploadRequest{
		RemoteAddress: remoteAddress,
		Body:          bytes.NewReader(buffer.Bytes()),
		Size:          int64(buffer.Len()),
		ContentType:   "application/json",
		Checksum:      this.hasher.Sum(nil),
	}
}

func (this *UploadApp) buildRemoteStorageClient() {
	client := shell.NewHTTPClient()
	gcsClient := shell.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, http.StatusOK)
	this.client = core.NewRetryClient(gcsClient, this.config.MaxRetry, time.Sleep)
}

func (this *UploadApp) completeManifest() {
	fileInfo, err := os.Stat(this.file.Name())
	if err != nil {
		log.Fatal(err)
	}
	this.manifest = contracts.Manifest{
		Name:    this.packageConfig.PackageName,
		Version: this.packageConfig.PackageVersion,
		Archive: contracts.Archive{
			Filename:             contracts.RemoteArchiveFilename,
			Size:                 uint64(fileInfo.Size()),
			MD5Checksum:          this.hasher.Sum(nil),
			Contents:             this.builder.Contents(),
			CompressionAlgorithm: this.packageConfig.CompressionAlgorithm,
		},
	}
}

func (this *UploadApp) closeArchiveFile() {
	err := this.file.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func (this *UploadApp) deleteLocalArchiveFile() {
	err := os.Remove(this.file.Name())
	if err != nil {
		log.Fatal(err)
	}
}

func (this *UploadApp) openArchiveFile() {
	var err error
	this.file, err = os.Open(this.file.Name())
	if err != nil {
		log.Fatal(err)
	}
}

func (this *UploadApp) upload(request contracts.UploadRequest) {
	err := this.client.Upload(request)
	if err != nil {
		log.Fatal(err)
	}
}

func (this *UploadApp) writeManifestToBuffer() *bytes.Buffer {
	buffer := new(bytes.Buffer)
	this.hasher.Reset()
	writer := io.MultiWriter(buffer, this.hasher)
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(this.manifest)
	return buffer
}

func (this *UploadApp) dumpManifest() string {
	raw, err := json.MarshalIndent(this.manifest, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	return "\n" + string(raw)
}
