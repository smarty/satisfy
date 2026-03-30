package transfer

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/internal/core"
	"github.com/smarty/satisfy/internal/plumbing"
	"github.com/smarty/satisfy/internal/shell"
)

const ForceAccessTokenRefreshInSeconds = 1800

type UploadApp struct {
	config        contracts.UploadConfiguration
	packageConfig contracts.PackageConfig
	file          *os.File
	hasher        hash.Hash
	compressor    io.WriteCloser
	builder       core.PackageBuilder
	manifest      plumbing.Manifest
	client        plumbing.RemoteStorage
}

func NewUploadApp(config contracts.UploadConfiguration) *UploadApp {
	return &UploadApp{config: config, packageConfig: config.PackageConfig}
}

func (this *UploadApp) Run(yield func(contracts.Event, error) bool) {
	emit := func(e contracts.Event) { yield(e, nil) }

	if !this.config.Overwrite {
		client := shell.NewHTTPClient()
		gcsClient := shell.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, []int{http.StatusNotFound})
		retryClient := core.NewRetryClient(gcsClient, this.config.MaxRetry, time.Sleep, emit)
		address := this.config.PackageConfig.ComposeRemoteAddress(contracts.RemoteManifestFilename)
		body, err := retryClient.Download(address)
		if err == nil {
			_ = body.Close()
		} else if code, ok := contracts.StatusCode(err); ok && code == http.StatusOK {
			yield(contracts.Event{}, contracts.ErrPackageExists)
			return
		} else {
			yield(contracts.Event{}, fmt.Errorf("sanity check failed: %w", err))
			return
		}
	}

	this.buildRemoteStorageClient(emit)
	start := time.Now().UTC()

	if err := this.buildArchiveAndManifestContents(emit); err != nil {
		yield(contracts.Event{}, err)
		return
	}

	if err := this.completeManifest(); err != nil {
		yield(contracts.Event{}, err)
		return
	}

	manifestStr, err := this.dumpManifest()
	if err != nil {
		yield(contracts.Event{}, err)
		return
	}

	if !yield(contracts.Event{Type: contracts.EventInfo, Message: "Manifest: " + manifestStr}, nil) {
		return
	}

	if time.Now().UTC().Sub(start).Milliseconds() > ForceAccessTokenRefreshInSeconds {
		creds, refreshErr := this.config.CredentialReader.Read(context.Background(), "")
		this.config.GoogleCredentials = creds
		this.buildRemoteStorageClient(emit)
		if refreshErr != nil {
			if !yield(contracts.Event{Type: contracts.EventWarning, Message: fmt.Sprintf("Cannot refresh token: %v", refreshErr)}, nil) {
				return
			}
		}
	}

	if !yield(contracts.Event{Type: contracts.EventInfo, Message: "Uploading the archive..."}, nil) {
		return
	}

	archiveReq, err := this.buildArchiveUploadRequest()
	if err != nil {
		yield(contracts.Event{}, err)
		return
	}

	if err = this.client.Upload(archiveReq); err != nil {
		yield(contracts.Event{}, err)
		return
	}

	if err = this.closeArchiveFile(); err != nil {
		yield(contracts.Event{}, err)
		return
	}

	if err = this.deleteLocalArchiveFile(); err != nil {
		yield(contracts.Event{}, err)
		return
	}

	if !yield(contracts.Event{Type: contracts.EventInfo, Message: "Uploading the manifest..."}, nil) {
		return
	}

	if err = this.client.Upload(this.buildManifestUploadRequest(this.packageConfig.ComposeRemoteAddress(contracts.RemoteManifestFilename))); err != nil {
		yield(contracts.Event{}, err)
		return
	}

	if err = this.client.Upload(this.buildManifestUploadRequest(this.packageConfig.ComposeLatestManifestRemoteAddress())); err != nil {
		yield(contracts.Event{}, err)
		return
	}
}

func (this *UploadApp) buildArchiveAndManifestContents(emit func(contracts.Event)) error {
	var err error
	this.file, err = os.CreateTemp("", "")
	if err != nil {
		return err
	}

	this.hasher = md5.New()
	writer := io.MultiWriter(this.hasher, this.file)

	if err = this.initializeCompressor(writer); err != nil {
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
		shell.NewSwitchArchiveWriter(this.compressor),
		md5.New(),
		this.config.NewProgress,
		emit,
	)

	if err = this.builder.Build(); err != nil {
		return err
	}

	if err = this.compressor.Close(); err != nil {
		return err
	}

	return this.closeArchiveFile()
}

func (this *UploadApp) initializeCompressor(writer io.Writer) error {
	factory, found := compression[this.packageConfig.CompressionAlgorithm]
	if !found {
		return fmt.Errorf("unsupported compression algorithm: %s", this.packageConfig.CompressionAlgorithm)
	}

	var err error
	this.compressor, err = factory(writer, this.packageConfig.CompressionLevel)
	return err
}

var compression = map[string]func(_ io.Writer, level int) (io.WriteCloser, error){
	"zstd": func(writer io.Writer, level int) (io.WriteCloser, error) {
		return zstd.NewWriter(writer, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)))
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

func (this *UploadApp) buildArchiveUploadRequest() (plumbing.UploadRequest, error) {
	if err := this.openArchiveFile(); err != nil {
		return plumbing.UploadRequest{}, err
	}

	return plumbing.UploadRequest{
		RemoteAddress: this.packageConfig.ComposeRemoteAddress(contracts.RemoteArchiveFilename),
		Body:          NewFileWrapper(this.file),
		Size:          int64(this.manifest.Archive.Size),
		ContentType:   contentType[this.manifest.Archive.CompressionAlgorithm],
		Checksum:      this.manifest.Archive.MD5Checksum,
	}, nil
}

func (this *UploadApp) buildManifestUploadRequest(remoteAddress url.URL) plumbing.UploadRequest {
	buffer := this.writeManifestToBuffer()
	return plumbing.UploadRequest{
		RemoteAddress: remoteAddress,
		Body:          bytes.NewReader(buffer.Bytes()),
		Size:          int64(buffer.Len()),
		ContentType:   "application/json",
		Checksum:      this.hasher.Sum(nil),
	}
}

func (this *UploadApp) buildRemoteStorageClient(emit func(contracts.Event)) {
	client := shell.NewHTTPClient()
	gcsClient := shell.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, []int{http.StatusOK})
	this.client = core.NewRetryClient(gcsClient, this.config.MaxRetry, time.Sleep, emit)
}

func (this *UploadApp) completeManifest() error {
	fileInfo, err := os.Stat(this.file.Name())
	if err != nil {
		return err
	}

	this.manifest = plumbing.Manifest{
		Name:    this.packageConfig.PackageName,
		Version: this.packageConfig.PackageVersion,
		Archive: plumbing.Archive{
			Filename:             contracts.RemoteArchiveFilename,
			Size:                 uint64(fileInfo.Size()),
			MD5Checksum:          this.hasher.Sum(nil),
			Contents:             this.builder.Contents(),
			CompressionAlgorithm: this.packageConfig.CompressionAlgorithm,
		},
	}

	return nil
}

func (this *UploadApp) closeArchiveFile() error {
	return this.file.Close()
}

func (this *UploadApp) deleteLocalArchiveFile() error {
	return os.Remove(this.file.Name())
}

func (this *UploadApp) openArchiveFile() error {
	var err error
	this.file, err = os.Open(this.file.Name())
	return err
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

func (this *UploadApp) dumpManifest() (string, error) {
	raw, err := json.MarshalIndent(this.manifest, "", "  ")
	if err != nil {
		return "", err
	}

	return "\n" + string(raw), nil
}
