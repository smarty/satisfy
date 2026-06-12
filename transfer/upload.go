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
	"github.com/smarty/satisfy/core"
	"github.com/smarty/satisfy/shell"
)

const ForceAccessTokenRefreshInSeconds = 1800

type UploadApp struct {
	config        contracts.UploadConfig
	packageConfig contracts.PackageConfig
	file          *os.File
	hasher        hash.Hash
	compressor    io.WriteCloser
	builder       core.PackageBuilder
	manifest      contracts.Manifest
	client        contracts.RemoteStorage
}

func NewUploadApp(config contracts.UploadConfig) *UploadApp {
	NewCheckApp(config).Run()
	return &UploadApp{config: config, packageConfig: config.PackageConfig}
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
	this.upload(this.buildManifestUploadRequest(this.manifest, this.packageConfig.ComposeRemoteAddress(contracts.RemoteManifestFilename)))
	this.upload(this.buildManifestUploadRequest(this.buildRootManifest(), this.packageConfig.ComposeLatestManifestRemoteAddress()))
}

// buildRootManifest carries the tag listing forward from the existing root
// manifest and points any tags named in the upload config at the version being
// uploaded. The versioned manifest never includes tags.
func (this *UploadApp) buildRootManifest() contracts.Manifest {
	rootManifest := this.manifest
	rootManifest.Tags = core.MergeTags(this.downloadExistingTags(), this.packageConfig.Tags, this.packageConfig.PackageVersion)
	for _, name := range this.packageConfig.Tags {
		log.Printf("Tagging version %q as %q", this.packageConfig.PackageVersion, name)
	}
	return rootManifest
}

func (this *UploadApp) downloadExistingTags() []contracts.Tag {
	body, err := this.client.Download(this.packageConfig.ComposeLatestManifestRemoteAddress())
	if contracts.IsNotFound(err) {
		return nil // first upload of this package
	}
	if err != nil {
		log.Fatal("Could not download existing root manifest (needed to preserve tags): ", err)
	}
	defer func() { _ = body.Close() }()

	var existing contracts.Manifest
	if err = json.NewDecoder(body).Decode(&existing); err != nil {
		log.Fatal("Could not decode existing root manifest (needed to preserve tags): ", err)
	}
	return existing.Tags
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
		sourcePath = this.config.PackageConfig.SourceFile
	}

	this.builder = core.NewDirectoryPackageBuilder(
		shell.NewDiskFileSystem(sourcePath),
		shell.NewSwitchArchiveWriter(this.compressor),
		md5.New(),
		this.config.ShowProgress,
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

func (this *UploadApp) buildManifestUploadRequest(manifest contracts.Manifest, remoteAddress url.URL) contracts.UploadRequest {
	buffer := this.writeManifestToBuffer(manifest)
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
	gcsClient := shell.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, []int{http.StatusOK})
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

func (this *UploadApp) writeManifestToBuffer(manifest contracts.Manifest) *bytes.Buffer {
	buffer := new(bytes.Buffer)
	this.hasher.Reset()
	writer := io.MultiWriter(buffer, this.hasher)
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(manifest)
	return buffer
}

func (this *UploadApp) dumpManifest() string {
	raw, err := json.MarshalIndent(this.manifest, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	return "\n" + string(raw)
}
