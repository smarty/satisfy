package satisfy

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/smarty/satisfy/v2/internal/model"
	"github.com/smarty/satisfy/v2/internal/storage"
)

// Client installs, uploads and inspects packages in Google Cloud Storage. It
// is safe for concurrent use.
type Client struct {
	storage        model.RemoteStorage
	concurrency    int
	verifyContents bool
	logger         *slog.Logger
	progress       model.ProgressFunc
}

// New returns a Client. It performs no I/O.
func New(options ...Option) *Client {
	config := defaultClientConfig()
	for _, option := range options {
		option(&config)
	}
	logger := model.LoggerOrDiscard(config.logger)
	remote := config.storage
	if remote == nil {
		remote = storage.NewGoogleCloudStorageClient(config.httpClient, config.credentials,
			[]int{http.StatusOK, http.StatusPartialContent})
	}
	return &Client{
		storage:        storage.NewRetryClient(remote, config.maxRetry, config.retryDelay, logger),
		concurrency:    config.concurrency,
		verifyContents: config.verifyContents,
		logger:         logger,
		progress:       config.progress,
	}
}

func (this *Client) downloadManifest(ctx context.Context, address url.URL) (manifest model.Manifest, err error) {
	manifest, _, err = this.downloadManifestGeneration(ctx, address)
	return manifest, err
}

// downloadManifestGeneration also returns the generation to make a later
// upload conditional on: 0 when the manifest does not exist, or nil when the
// storage does not report generations.
func (this *Client) downloadManifestGeneration(ctx context.Context, address url.URL) (manifest model.Manifest, generation *int64, err error) {
	body, err := this.storage.Download(ctx, address)
	if model.IsNotFound(err) {
		return model.Manifest{}, new(int64), err
	}
	if err != nil {
		return model.Manifest{}, nil, err
	}
	defer func() { _ = body.Close() }()
	if generational, ok := body.(model.Generational); ok {
		generation = new(generational.Generation())
	}
	if err = json.UnmarshalRead(body, &manifest); err != nil {
		return model.Manifest{}, nil, fmt.Errorf("could not decode manifest at [%s]: %w", address.String(), err)
	}
	return manifest, generation, nil
}

// rootManifestUpdateAttempts bounds how often a root manifest update starts
// over after a concurrent change.
const rootManifestUpdateAttempts = 5

// updateRootManifest reads the root manifest at address, passes it to change
// (found reports whether it exists) and writes the result back only if the
// root manifest has not changed in the meantime, so that concurrent uploads
// and tag modifications don't lose each other's tags. On a concurrent change
// it starts over.
func (this *Client) updateRootManifest(ctx context.Context, address url.URL, change func(existing model.Manifest, found bool) (model.Manifest, error)) error {
	for attempt := 1; ; attempt++ {
		existing, generation, err := this.downloadManifestGeneration(ctx, address)
		if err != nil && !model.IsNotFound(err) {
			return fmt.Errorf("could not read root manifest at [%s]: %w", address.String(), err)
		}
		updated, err := change(existing, err == nil)
		if err != nil {
			return err
		}
		err = this.uploadManifest(ctx, updated, address, generation)
		if errors.Is(err, model.ErrPreconditionFailed) && attempt < rootManifestUpdateAttempts {
			this.logger.Info("root manifest changed concurrently; retrying", "address", address.String())
			continue
		}
		return err
	}
}

// uploadManifest writes manifest to address; a non-nil ifGenerationMatch
// makes the write conditional (see model.UploadRequest).
func (this *Client) uploadManifest(ctx context.Context, manifest model.Manifest, address url.URL, ifGenerationMatch *int64) error {
	buffer := new(bytes.Buffer)
	hasher := md5.New()
	if err := json.MarshalWrite(io.MultiWriter(buffer, hasher), manifest, jsontext.WithIndent("  ")); err != nil {
		return err
	}
	return this.storage.Upload(ctx, model.UploadRequest{
		RemoteAddress:     address,
		Body:              bytes.NewReader(buffer.Bytes()),
		Size:              int64(buffer.Len()),
		ContentType:       "application/json",
		Checksum:          hasher.Sum(nil),
		IfGenerationMatch: ifGenerationMatch,
	})
}
