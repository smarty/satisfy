package storage

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"time"

	"github.com/smarty/satisfy/v2/internal/model"
)

// DefaultRetryDelay is the pause between attempts of a retryable operation.
const DefaultRetryDelay = 3 * time.Second

// RetryClient retries operations that fail with model.ErrRetryable, up to
// maxRetry additional attempts, pausing between attempts. A cancelled context
// stops the retries.
type RetryClient struct {
	inner    model.RemoteStorage
	maxRetry int
	delay    time.Duration
	logger   *slog.Logger
	wait     func(ctx context.Context, delay time.Duration) error
}

func NewRetryClient(inner model.RemoteStorage, maxRetry int, delay time.Duration, logger *slog.Logger) *RetryClient {
	return &RetryClient{
		inner:    inner,
		maxRetry: maxRetry,
		delay:    delay,
		logger:   model.LoggerOrDiscard(logger),
		wait:     waitContext,
	}
}

func (this *RetryClient) Upload(ctx context.Context, request model.UploadRequest) error {
	return this.retry(ctx, "upload", func() error {
		if request.Body != nil {
			if _, err := request.Body.Seek(0, io.SeekStart); err != nil {
				return err
			}
		}
		return this.inner.Upload(ctx, request)
	})
}

func (this *RetryClient) Download(ctx context.Context, address url.URL) (body io.ReadCloser, err error) {
	err = this.retry(ctx, "download", func() (err error) {
		body, err = this.inner.Download(ctx, address)
		return err
	})
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (this *RetryClient) Seek(ctx context.Context, address url.URL, start, end int64) (body io.ReadCloser, err error) {
	err = this.retry(ctx, "seek", func() (err error) {
		body, err = this.inner.Seek(ctx, address, start, end)
		return err
	})
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (this *RetryClient) Size(ctx context.Context, address url.URL) (size int64, err error) {
	err = this.retry(ctx, "size", func() (err error) {
		size, err = this.inner.Size(ctx, address)
		return err
	})
	if err != nil {
		return 0, err
	}
	return size, nil
}

func (this *RetryClient) retry(ctx context.Context, operation string, attempt func() error) (err error) {
	for x := 0; x <= this.maxRetry; x++ {
		err = attempt()
		if err == nil || !errors.Is(err, model.ErrRetryable) {
			return err
		}
		if x < this.maxRetry {
			this.logger.Warn("remote storage operation failed; retrying", "operation", operation, "attempt", x+1, "error", err)
			if waitErr := this.wait(ctx, this.delay); waitErr != nil {
				return waitErr
			}
		}
	}
	return err
}

func waitContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
