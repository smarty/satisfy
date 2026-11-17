package contracts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type RemoteStorage interface {
	Uploader
	Downloader
}

type Uploader interface {
	Upload(context.Context, UploadRequest) error
}

type UploadRequest struct {
	RemoteAddress url.URL
	Body          io.ReadSeeker
	Size          int64
	ContentType   string
	Checksum      []byte
}

type Downloader interface {
	Download(context.Context, url.URL) (io.ReadCloser, error)
	Seek(ctx context.Context, address url.URL, start, end int64) (io.ReadCloser, error)
	Size(context.Context, url.URL) (int64, error)
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

var ErrRetryable = errors.New("retry")

// ErrPackageExists indicates that the package version being uploaded is
// already present on remote storage.
var ErrPackageExists = errors.New("package already exists on remote storage")

func IsNotFound(err error) bool {
	statusError, ok := errors.AsType[*StatusCodeError](err)
	return ok && statusError.StatusCode() == http.StatusNotFound
}

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
