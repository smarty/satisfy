package satisfy

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json/jsontext"
	"encoding/json/v2"
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

func (this *Client) downloadManifest(ctx context.Context, address url.URL) (manifest Manifest, err error) {
	body, err := this.storage.Download(ctx, address)
	if err != nil {
		return Manifest{}, err
	}
	defer func() { _ = body.Close() }()
	if err = json.UnmarshalRead(body, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("could not decode manifest at [%s]: %w", address.String(), err)
	}
	return manifest, nil
}

func (this *Client) uploadManifest(ctx context.Context, manifest Manifest, address url.URL) error {
	buffer := new(bytes.Buffer)
	hasher := md5.New()
	if err := json.MarshalWrite(io.MultiWriter(buffer, hasher), manifest, jsontext.WithIndent("  ")); err != nil {
		return err
	}
	return this.storage.Upload(ctx, model.UploadRequest{
		RemoteAddress: address,
		Body:          bytes.NewReader(buffer.Bytes()),
		Size:          int64(buffer.Len()),
		ContentType:   "application/json",
		Checksum:      hasher.Sum(nil),
	})
}
