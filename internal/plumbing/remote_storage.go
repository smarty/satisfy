package plumbing

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
