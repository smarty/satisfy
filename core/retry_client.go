package core

import (
	"errors"
	"io"
	"net/url"
	"time"

	"github.com/smartystreets/clock"
	"github.com/smartystreets/logging"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type RetryClient struct {
	sleeper  *clock.Sleeper
	logger   *logging.Logger
	inner    contracts.RemoteStorage
	maxRetry int
}

func NewRetryClient(inner contracts.RemoteStorage, maxRetry int) *RetryClient {
	return &RetryClient{inner: inner, maxRetry: maxRetry}
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
			this.logger.Println("[WARN] upload failed, retry imminent.")
			this.sleeper.Sleep(time.Second * 3)
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
			this.logger.Println("[WARN] download failed, retry imminent.")
			this.sleeper.Sleep(time.Second * 3)
		}
	}
	return nil, err
}
