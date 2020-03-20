package contracts

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
)

type RemoteStorage interface {
	Uploader
	Downloader
}

type Uploader interface {
	Upload(UploadRequest) error
}

type UploadRequest struct {
	RemoteAddress url.URL
	Body          io.ReadSeeker
	Size          int64
	ContentType   string
	Checksum      []byte
}

type Downloader interface {
	Download(url.URL) (io.ReadCloser, error)
}

func AppendRemotePath(prefix url.URL, packageName, version, fileName string) url.URL {
	if version == "latest" {
		prefix.Path = path.Join(prefix.Path, packageName, fileName)
	} else {
		prefix.Path = path.Join(prefix.Path, packageName, version, fileName)
	}
	if !strings.HasPrefix(prefix.Path, "/") {
		prefix.Path = "/" + prefix.Path
	}
	return prefix
}

var RetryErr = errors.New("retry")

type StatusCodeError struct {
	actualStatusCode   int
	expectedStatusCode int
	remoteAddress      url.URL
}

func NewStatusCodeError(actual int, expected int, remoteAddress url.URL) *StatusCodeError {
	return &StatusCodeError{actualStatusCode: actual, expectedStatusCode: expected, remoteAddress: remoteAddress}
}

func (this *StatusCodeError) Error() string {
	return fmt.Sprintf(
		"expected status code: [%d] actual status code: [%d] remote address: [%s]",
		this.expectedStatusCode, this.actualStatusCode, this.remoteAddress.String(),
	)
}

func (this *StatusCodeError) StatusCode() int {
	return this.actualStatusCode
}
