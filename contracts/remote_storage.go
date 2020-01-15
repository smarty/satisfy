package contracts

import "io"

type RemoteStorage interface {
	Uploader
	Downloader
}

type Uploader interface {
	Upload(UploadRequest) error
}

type UploadRequest struct {
	Bucket      string
	Resource    string
	Body        io.ReadSeeker
	Size        int64
	ContentType string
	Checksum    []byte
}

type Downloader interface {
	Download(DownloadRequest) (io.ReadCloser, error)
}

type DownloadRequest struct {
	Bucket   string
	Resource string
}
