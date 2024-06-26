package core

import (
	"errors"
	"io"
	"log"
	"net/url"
	"time"

	"github.com/smarty/satisfy/contracts"
)

type RetryClient struct {
	inner    contracts.RemoteStorage
	maxRetry int
	sleep    func(duration time.Duration)
}

func NewRetryClient(inner contracts.RemoteStorage, maxRetry int, sleep func(duration time.Duration)) *RetryClient {
	return &RetryClient{inner: inner, maxRetry: maxRetry, sleep: sleep}
}

func (this *RetryClient) Upload(request contracts.UploadRequest) (err error) {
	for x := 0; x <= this.maxRetry; x++ {
		err = this.inner.Upload(request)
		if err == nil {
			return nil
		}
		if !errors.Is(err, contracts.RetryErr) {
			return err
		}
		if x < this.maxRetry {
			log.Printf("[WARN] upload failed; retry imminent: %v", err)
			this.sleep(time.Second * 3)
		}
	}
	return err
}

func (this *RetryClient) Download(request url.URL) (body io.ReadCloser, err error) {
	for x := 0; x <= this.maxRetry; x++ {
		body, err = this.inner.Download(request)
		if err == nil {
			return body, nil
		}
		if !errors.Is(err, contracts.RetryErr) {
			return nil, err
		}
		if x < this.maxRetry {
			log.Println("[WARN] download failed, retry imminent.")
			this.sleep(time.Second * 3)
		}
	}
	return nil, err
}

func (this *RetryClient) Seek(request url.URL, start, end int64) (body io.ReadCloser, err error) {
	for x := 0; x <= this.maxRetry; x++ {
		body, err = this.inner.Seek(request, start, end)
		if err == nil {
			return body, nil
		}
		if !errors.Is(err, contracts.RetryErr) {
			return nil, err
		}
		if x < this.maxRetry {
			log.Println("[WARN] seek failed, retry imminent.")
			this.sleep(time.Second * 3)
		}
	}
	return nil, err
}

func (this *RetryClient) Size(request url.URL) (size int64, err error) {
	for x := 0; x <= this.maxRetry; x++ {
		size, err = this.inner.Size(request)
		if err == nil {
			return size, nil
		}
		if !errors.Is(err, contracts.RetryErr) {
			return 0, err
		}
		if x < this.maxRetry {
			log.Println("[WARN] size failed, retry imminent.")
			this.sleep(time.Second * 3)
		}
	}
	return 0, err
}
