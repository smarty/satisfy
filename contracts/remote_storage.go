package contracts

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
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
	Seek(url.URL, int64, int64) (io.ReadCloser, error)
	Size(url.URL) (int64, error)
}

type DownloadSetter interface {
	SetDownloader(url.URL, Downloader)
}

var RetryErr = errors.New("retry")

type StatusCodeError struct {
	actualStatusCode   int
	expectedStatusCode []int
	remoteAddress      url.URL
}

func NewStatusCodeError(actual int, expected []int, remoteAddress url.URL) *StatusCodeError {
	return &StatusCodeError{actualStatusCode: actual, expectedStatusCode: expected, remoteAddress: remoteAddress}
}

func (this *StatusCodeError) Error() string {
	var IDs []string
	for _, i := range this.expectedStatusCode {
		IDs = append(IDs, strconv.Itoa(i))
	}

	return fmt.Sprintf(
		"expected status code: [%s] actual status code: [%d] remote address: [%s]",
		strings.Join(IDs, " or "), this.actualStatusCode, this.remoteAddress.String(),
	)
}

func (this *StatusCodeError) StatusCode() int {
	return this.actualStatusCode
}
