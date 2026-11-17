package transfer

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"github.com/smarty/satisfy/v2/contracts"
	"github.com/smarty/satisfy/v2/core"
	"github.com/smarty/satisfy/v2/shell"
)

// accessTokenRefreshThreshold is how long archive compression may run before the
// bearer token is refreshed, so that the upload itself does not outlive it.
const accessTokenRefreshThreshold = 30 * time.Minute

type UploadApp struct {
	config        contracts.UploadConfig
	packageConfig contracts.PackageConfig
	file          *os.File
	hasher        hash.Hash
	builder       core.PackageBuilder
	manifest      contracts.Manifest
	client        contracts.RemoteStorage
}

// NewUploadApp prepares an upload. It performs no I/O; callers that want to
// avoid overwriting an existing package should run a CheckApp first.
func NewUploadApp(config contracts.UploadConfig) *UploadApp {
	return &UploadApp{config: config, packageConfig: config.PackageConfig}
}

func (this *UploadApp) Run(ctx context.Context) (err error) {
	this.buildRemoteStorageClient()

	start := time.Now().UTC()

	if err = this.buildArchiveAndManifestContents(ctx); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, this.deleteLocalArchiveFile()) }()

	if err = this.completeManifest(); err != nil {
		return err
	}

	log.Println("Manifest:", this.dumpManifest())

	if needsAccessTokenRefresh(start, time.Now().UTC()) && this.config.CredentialReader != nil {
		credentials, err := this.config.CredentialReader.Read(ctx, "")
		if err != nil {
			log.Println("[Error] Cannot refresh token: ", err)
		} else {
			this.config.GoogleCredentials = credentials
			this.buildRemoteStorageClient()
		}
	}

	log.Println("Uploading the archive...")
	if err = this.uploadArchive(ctx); err != nil {
		return err
	}

	log.Println("Uploading the manifest...")
	if err = this.uploadManifest(ctx, this.manifest, this.packageConfig.ComposeRemoteAddress(contracts.RemoteManifestFilename)); err != nil {
		return err
	}
	rootManifest, err := this.buildRootManifest(ctx)
	if err != nil {
		return err
	}
	return this.uploadManifest(ctx, rootManifest, this.packageConfig.ComposeLatestManifestRemoteAddress())
}

func needsAccessTokenRefresh(start, now time.Time) bool {
	return now.Sub(start) > accessTokenRefreshThreshold
}

// buildRootManifest carries the tag listing forward from the existing root
// manifest and points any tags named in the upload config at the version being
// uploaded. The versioned manifest never includes tags.
func (this *UploadApp) buildRootManifest(ctx context.Context) (contracts.Manifest, error) {
	existing, err := this.downloadExistingTags(ctx)
	if err != nil {
		return contracts.Manifest{}, err
	}
	rootManifest := this.manifest
	rootManifest.Tags = core.MergeTags(existing, this.packageConfig.Tags, this.packageConfig.PackageVersion)
	for _, name := range this.packageConfig.Tags {
		log.Printf("Tagging version %q as %q", this.packageConfig.PackageVersion, name)
	}
	return rootManifest, nil
}

func (this *UploadApp) downloadExistingTags(ctx context.Context) ([]contracts.Tag, error) {
	body, err := this.client.Download(ctx, this.packageConfig.ComposeLatestManifestRemoteAddress())
	if contracts.IsNotFound(err) {
		return nil, nil // first upload of this package
	}
	if err != nil {
		return nil, fmt.Errorf("could not download existing root manifest (needed to preserve tags): %w", err)
	}
	defer func() { _ = body.Close() }()

	var existing contracts.Manifest
	if err = json.NewDecoder(body).Decode(&existing); err != nil {
		return nil, fmt.Errorf("could not decode existing root manifest (needed to preserve tags): %w", err)
	}
	return existing.Tags, nil
}

func (this *UploadApp) uploadArchive(ctx context.Context) error {
	file, err := os.Open(this.file.Name())
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return this.client.Upload(ctx, contracts.UploadRequest{
		RemoteAddress: this.packageConfig.ComposeRemoteAddress(contracts.RemoteArchiveFilename),
		Body:          NewFileWrapper(file),
		Size:          int64(this.manifest.Archive.Size),
		ContentType:   contentType[this.manifest.Archive.CompressionAlgorithm],
		Checksum:      this.manifest.Archive.MD5Checksum,
	})
}

func (this *UploadApp) buildArchiveAndManifestContents(ctx context.Context) (err error) {
	this.file, err = os.CreateTemp("", "satisfy-archive-*")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, this.file.Close()) }()

	this.hasher = md5.New()
	compressor, err := newCompressor(this.packageConfig.CompressionAlgorithm, this.packageConfig.CompressionLevel, io.MultiWriter(this.hasher, this.file))
	if err != nil {
		return err
	}

	sourcePath := this.packageConfig.SourcePath
	if sourcePath == "" {
		sourcePath = this.packageConfig.SourceDirectory
	}
	if sourcePath == "" {
		sourcePath = this.packageConfig.SourceFile
	}

	this.builder = core.NewDirectoryPackageBuilder(
		shell.NewDiskFileSystem(sourcePath),
		shell.NewSwitchArchiveWriter(compressor),
		md5.New(),
		this.config.ShowProgress,
	)

	if err = this.builder.Build(ctx); err != nil {
		_ = compressor.Close()
		return err
	}
	return compressor.Close()
}

func newCompressor(algorithm string, level int, writer io.Writer) (io.WriteCloser, error) {
	factory, found := compression[algorithm]
	if !found {
		return nil, fmt.Errorf("unsupported compression algorithm: %q", algorithm)
	}
	return factory(writer, level)
}

var compression = map[string]func(_ io.Writer, level int) (io.WriteCloser, error){
	"zstd": func(writer io.Writer, level int) (io.WriteCloser, error) {
		return shell.NewParallelZstdWriter(writer, zstd.EncoderLevelFromZstd(level), shell.DefaultParallelZstdChunkSize)
	},
	"gzip": func(writer io.Writer, level int) (io.WriteCloser, error) {
		return gzip.NewWriterLevel(writer, level)
	},
	"zip": func(writer io.Writer, level int) (io.WriteCloser, error) {
		return shell.NewZipArchiveWriter(writer, level), nil
	},
}
var contentType = map[string]string{
	"zstd": "application/zstd",
	"gzip": "application/gzip",
	"zip":  "application/zip",
}

func (this *UploadApp) uploadManifest(ctx context.Context, manifest contracts.Manifest, remoteAddress url.URL) error {
	buffer := new(bytes.Buffer)
	hasher := md5.New()
	encoder := json.NewEncoder(io.MultiWriter(buffer, hasher))
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		return err
	}
	return this.client.Upload(ctx, contracts.UploadRequest{
		RemoteAddress: remoteAddress,
		Body:          bytes.NewReader(buffer.Bytes()),
		Size:          int64(buffer.Len()),
		ContentType:   "application/json",
		Checksum:      hasher.Sum(nil),
	})
}

func (this *UploadApp) buildRemoteStorageClient() {
	gcsClient := shell.NewGoogleCloudStorageClient(shell.NewHTTPClient(), this.config.GoogleCredentials, []int{http.StatusOK})
	this.client = core.NewRetryClient(gcsClient, this.config.MaxRetry, time.Sleep)
}

func (this *UploadApp) completeManifest() error {
	fileInfo, err := os.Stat(this.file.Name())
	if err != nil {
		return err
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
	return nil
}

func (this *UploadApp) deleteLocalArchiveFile() error {
	if this.file == nil {
		return nil
	}
	return os.Remove(this.file.Name())
}

func (this *UploadApp) dumpManifest() string {
	raw, _ := json.MarshalIndent(this.manifest, "", "  ") // a Manifest always marshals
	return "\n" + string(raw)
}
