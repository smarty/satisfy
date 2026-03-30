package core

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/internal/plumbing"
)

type RetryClient struct {
	inner    plumbing.RemoteStorage
	maxRetry int
	sleep    func(duration time.Duration)
	emit     func(contracts.Event)
}

func NewRetryClient(inner plumbing.RemoteStorage, maxRetry int, sleep func(duration time.Duration), emit func(contracts.Event)) *RetryClient {
	if emit == nil {
		emit = func(contracts.Event) {}
	}
	return &RetryClient{inner: inner, maxRetry: maxRetry, sleep: sleep, emit: emit}
}

func (this *RetryClient) Upload(request plumbing.UploadRequest) (err error) {
	for x := 0; x <= this.maxRetry; x++ {
		err = this.inner.Upload(request)
		if err == nil {
			return nil
		}
		if !errors.Is(err, contracts.ErrRetry) {
			return err
		}
		if x < this.maxRetry {
			this.emit(contracts.Event{Type: contracts.EventWarning, Message: fmt.Sprintf("upload failed; retry imminent: %v", err)})
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
		if !errors.Is(err, contracts.ErrRetry) {
			return nil, err
		}
		if x < this.maxRetry {
			this.emit(contracts.Event{Type: contracts.EventWarning, Message: "download failed, retry imminent."})
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
		if !errors.Is(err, contracts.ErrRetry) {
			return nil, err
		}
		if x < this.maxRetry {
			this.emit(contracts.Event{Type: contracts.EventWarning, Message: "seek failed, retry imminent."})
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
		if !errors.Is(err, contracts.ErrRetry) {
			return 0, err
		}
		if x < this.maxRetry {
			this.emit(contracts.Event{Type: contracts.EventWarning, Message: "size failed, retry imminent."})
			this.sleep(time.Second * 3)
		}
	}
	return 0, err
}
