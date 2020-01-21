package contracts

import (
	"io"
	"net/url"
	"path"
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
	prefix.Path = path.Join(prefix.Path, packageName, version, fileName)
	return prefix
}
