package remote

import (
	"time"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"github.com/smartystreets/clock"
	"github.com/smartystreets/logging"
)

type RetryUploader struct {
	sleeper  *clock.Sleeper
	logger   *logging.Logger
	inner    contracts.Uploader
	maxRetry int
}

func NewRetryUploader(inner contracts.Uploader, maxRetry int) *RetryUploader {
	return &RetryUploader{inner: inner, maxRetry: maxRetry}
}

func (this *RetryUploader) Upload(request contracts.UploadRequest) (err error) {
	for x := 0; x <= this.maxRetry; x++ {
		err = this.inner.Upload(request)
		if err == nil {
			return nil
		}
		if x < this.maxRetry {
			// TODO: request.Body.Seek(0, 0)
			this.logger.Println("[WARN] upload failed, retry imminent.")
			this.sleeper.Sleep(time.Second * 3)
		}
	}
	return err
}
