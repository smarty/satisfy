package core

import (
	"context"
	"errors"
	"io"
	"log"
	"net/url"
	"time"

	"github.com/smarty/satisfy/v2/contracts"
)

type RetryClient struct {
	inner    contracts.RemoteStorage
	maxRetry int
	sleep    func(duration time.Duration)
}

func NewRetryClient(inner contracts.RemoteStorage, maxRetry int, sleep func(duration time.Duration)) *RetryClient {
	return &RetryClient{inner: inner, maxRetry: maxRetry, sleep: sleep}
}

func (this *RetryClient) Upload(ctx context.Context, request contracts.UploadRequest) (err error) {
	for x := 0; x <= this.maxRetry; x++ {
		err = this.inner.Upload(ctx, request)
		if err == nil {
			return nil
		}
		if !errors.Is(err, contracts.ErrRetryable) {
			return err
		}
		if x < this.maxRetry {
			log.Printf("[WARN] upload failed; retry imminent: %v", err)
			this.sleep(time.Second * 3)
		}
	}
	return err
}

func (this *RetryClient) Download(ctx context.Context, request url.URL) (body io.ReadCloser, err error) {
	for x := 0; x <= this.maxRetry; x++ {
		body, err = this.inner.Download(ctx, request)
		if err == nil {
			return body, nil
		}
		if !errors.Is(err, contracts.ErrRetryable) {
			return nil, err
		}
		if x < this.maxRetry {
			log.Println("[WARN] download failed, retry imminent.")
			this.sleep(time.Second * 3)
		}
	}
	return nil, err
}

func (this *RetryClient) Seek(ctx context.Context, request url.URL, start, end int64) (body io.ReadCloser, err error) {
	for x := 0; x <= this.maxRetry; x++ {
		body, err = this.inner.Seek(ctx, request, start, end)
		if err == nil {
			return body, nil
		}
		if !errors.Is(err, contracts.ErrRetryable) {
			return nil, err
		}
		if x < this.maxRetry {
			log.Println("[WARN] seek failed, retry imminent.")
			this.sleep(time.Second * 3)
		}
	}
	return nil, err
}

func (this *RetryClient) Size(ctx context.Context, request url.URL) (size int64, err error) {
	for x := 0; x <= this.maxRetry; x++ {
		size, err = this.inner.Size(ctx, request)
		if err == nil {
			return size, nil
		}
		if !errors.Is(err, contracts.ErrRetryable) {
			return 0, err
		}
		if x < this.maxRetry {
			log.Println("[WARN] size failed, retry imminent.")
			this.sleep(time.Second * 3)
		}
	}
	return 0, err
}
