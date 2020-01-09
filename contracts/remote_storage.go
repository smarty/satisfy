package contracts

import "io"

type Uploader interface {
	Upload(UploadRequest) error
}

type UploadRequest struct {
	Path        string
	Body        io.ReadSeeker
	Size        int64
	ContentType string
	Checksum    []byte
}
