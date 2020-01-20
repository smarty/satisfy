package contracts

import (
	"io"
	"net/url"
)

type RemoteStorage interface {
	Uploader
	Downloader
}

type Uploader interface {
	Upload(UploadRequest) error
}

type UploadRequest struct {
	Bucket        string // deprecated
	Resource      string // deprecated
	RemoteAddress url.URL
	Body          io.ReadSeeker
	Size          int64
	ContentType   string
	Checksum      []byte
}

type Downloader interface {
	Download(DownloadRequest) (io.ReadCloser, error) // TODO this method will receive a URL
}

type DownloadRequest struct { // TODO delete this
	Bucket        string // deprecated
	Resource      string // deprecated
	RemoteAddress url.URL
}
