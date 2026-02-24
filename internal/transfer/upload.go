package transfer

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/json"
	"hash"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/internal/core"
	"github.com/smarty/satisfy/internal/shell"
	"github.com/smarty/satisfy/legacy_contracts"
)

const ForceAccessTokenRefreshInSeconds = 1800

type UploadApp struct {
	config        contracts.UploadConfiguration
	packageConfig contracts.PackageConfig
	file          *os.File
	hasher        hash.Hash
	compressor    io.WriteCloser
	builder       core.PackageBuilder
	manifest      legacy_contracts.Manifest
	client        legacy_contracts.RemoteStorage
}

func NewUploadApp(config contracts.UploadConfiguration) *UploadApp {
	runPreUploadCheck(config)
	return &UploadApp{config: config, packageConfig: config.PackageConfig}
}

func runPreUploadCheck(config contracts.UploadConfiguration) {
	if config.Overwrite {
		log.Println("[INFO] Overwrite mode enabled, skipping remote manifest check.")
		return
	}

	client := shell.NewHTTPClient()
	gcsClient := shell.NewGoogleCloudStorageClient(client, config.GoogleCredentials, []int{http.StatusNotFound})
	retryClient := core.NewRetryClient(gcsClient, config.MaxRetry, time.Sleep)

	address := config.PackageConfig.ComposeRemoteAddress(contracts.RemoteManifestFilename)
	body, err := retryClient.Download(address)
	if err == nil {
		_ = body.Close()
		return
	}

	statusError, ok := err.(*legacy_contracts.StatusCodeError)
	if ok && statusError.StatusCode() == http.StatusOK {
		log.Println("[INFO] Package already exists on remote storage.")
		os.Exit(2)
	}

	log.Fatalln("[WARN] Sanity check failed:", err)
}

func (this *UploadApp) Run() {
	this.buildRemoteStorageClient()

	start := time.Now().UTC()

	this.buildArchiveAndManifestContents()
	this.completeManifest()

	log.Println("Manifest:", this.dumpManifest())

	// TEMPORARY WORKAROUND
	// If the compression took over 30 minutes, we need to refresh the bearer token
	// so we have time to upload the archive and manifest before it expires.
	// Other Resolution Thoughts:
	// Perhaps the retry client can be modified to handle 401 errors and refresh the token,
	// The problem with this approach is if the file to upload is large, the 401 does not occur until after
	// the entire time is spent to upload the file, so it does not fail fast. If we attempted to upload
	// the manifest first it could be a fast fail.
	if time.Now().UTC().Sub(start).Milliseconds() > ForceAccessTokenRefreshInSeconds {
		var err error
		this.config.GoogleCredentials, err = this.config.CredentialReader.Read(context.Background(), "")
		this.buildRemoteStorageClient()
		if err != nil {
			log.Println("[Error] Cannot refresh token: ", err)
		}
	}

	log.Println("Uploading the archive...")
	this.upload(this.buildArchiveUploadRequest())
	this.closeArchiveFile()
	this.deleteLocalArchiveFile()

	log.Println("Uploading the manifest...")
	this.upload(this.buildManifestUploadRequest(this.packageConfig.ComposeRemoteAddress(contracts.RemoteManifestFilename)))
	this.upload(this.buildManifestUploadRequest(this.packageConfig.ComposeLatestManifestRemoteAddress()))
}

func (this *UploadApp) buildArchiveUploadRequest() legacy_contracts.UploadRequest {
	this.openArchiveFile()
	return legacy_contracts.UploadRequest{
		RemoteAddress: this.packageConfig.ComposeRemoteAddress(contracts.RemoteArchiveFilename),
		Body:          NewFileWrapper(this.file),
		Size:          int64(this.manifest.Archive.Size),
		ContentType:   contentType[this.manifest.Archive.CompressionAlgorithm],
		Checksum:      this.manifest.Archive.MD5Checksum,
	}
}

func (this *UploadApp) buildArchiveAndManifestContents() {
	var err error
	this.file, err = os.CreateTemp("", "")
	if err != nil {
		log.Fatal(err)
	}
	this.hasher = md5.New()
	writer := io.MultiWriter(this.hasher, this.file)
	this.InitializeCompressor(writer)

	sourcePath := this.packageConfig.SourcePath
	if sourcePath == "" {
		sourcePath = this.packageConfig.SourceDirectory
	}
	if sourcePath == "" {
		sourcePath = this.packageConfig.SourceFile
	}

	this.builder = core.NewDirectoryPackageBuilder(
		shell.NewDiskFileSystem(sourcePath),
		shell.NewSwitchArchiveWriter(this.compressor),
		md5.New(),
		this.config.NewProgress,
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

func (this *UploadApp) buildManifestUploadRequest(remoteAddress url.URL) legacy_contracts.UploadRequest {
	buffer := this.writeManifestToBuffer()
	return legacy_contracts.UploadRequest{
		RemoteAddress: remoteAddress,
		Body:          bytes.NewReader(buffer.Bytes()),
		Size:          int64(buffer.Len()),
		ContentType:   "application/json",
		Checksum:      this.hasher.Sum(nil),
	}
}

func (this *UploadApp) buildRemoteStorageClient() {
	client := shell.NewHTTPClient()
	gcsClient := shell.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, []int{http.StatusOK})
	this.client = core.NewRetryClient(gcsClient, this.config.MaxRetry, time.Sleep)
}

func (this *UploadApp) completeManifest() {
	fileInfo, err := os.Stat(this.file.Name())
	if err != nil {
		log.Fatal(err)
	}
	this.manifest = legacy_contracts.Manifest{
		Name:    this.packageConfig.PackageName,
		Version: this.packageConfig.PackageVersion,
		Archive: legacy_contracts.Archive{
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

func (this *UploadApp) upload(request legacy_contracts.UploadRequest) {
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
