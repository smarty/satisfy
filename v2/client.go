package satisfy

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/smarty/satisfy/v2/contracts"
	"github.com/smarty/satisfy/v2/core"
	"github.com/smarty/satisfy/v2/shell"
)

// Client installs, uploads and inspects packages in Google Cloud Storage. It
// is safe for concurrent use.
type Client struct {
	storage        contracts.RemoteStorage
	concurrency    int
	verifyContents bool
	logger         *slog.Logger
	progress       contracts.ProgressFunc
}

// New returns a Client. It performs no I/O.
func New(options ...Option) *Client {
	config := defaultClientConfig()
	for _, option := range options {
		option(&config)
	}
	logger := contracts.LoggerOrDiscard(config.logger)
	storage := config.storage
	if storage == nil {
		storage = shell.NewGoogleCloudStorageClient(config.httpClient, config.credentials,
			[]int{http.StatusOK, http.StatusPartialContent})
	}
	return &Client{
		storage:        core.NewRetryClient(storage, config.maxRetry, config.retryDelay, logger),
		concurrency:    config.concurrency,
		verifyContents: config.verifyContents,
		logger:         logger,
		progress:       config.progress,
	}
}

func (this *Client) downloadManifest(ctx context.Context, address url.URL) (manifest Manifest, err error) {
	body, err := this.storage.Download(ctx, address)
	if err != nil {
		return Manifest{}, err
	}
	defer func() { _ = body.Close() }()
	if err = json.NewDecoder(body).Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("could not decode manifest at [%s]: %w", address.String(), err)
	}
	return manifest, nil
}

func (this *Client) uploadManifest(ctx context.Context, manifest Manifest, address url.URL) error {
	buffer := new(bytes.Buffer)
	hasher := md5.New()
	encoder := json.NewEncoder(io.MultiWriter(buffer, hasher))
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		return err
	}
	return this.storage.Upload(ctx, contracts.UploadRequest{
		RemoteAddress: address,
		Body:          bytes.NewReader(buffer.Bytes()),
		Size:          int64(buffer.Len()),
		ContentType:   "application/json",
		Checksum:      hasher.Sum(nil),
	})
}
