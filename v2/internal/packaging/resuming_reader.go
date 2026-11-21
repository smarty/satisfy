package packaging

import (
	"context"
	"errors"
	"io"
	"net/url"

	"github.com/smarty/satisfy/v2/internal/model"
)

// maxDownloadResumes bounds how many times one download may resume after its
// connection fails mid-stream.
const maxDownloadResumes = 5

// resumingReader reads an object of known size and, when reading fails
// part-way through (e.g. the connection drops), requests the remainder with a
// ranged download instead of starting again from the first byte.
type resumingReader struct {
	ctx        context.Context
	downloader model.Downloader
	address    url.URL
	size       int64

	body    io.ReadCloser
	offset  int64
	resumes int
}

// NewResumingReader reads body, the download of the object of the given size
// at address, and resumes it with ranged downloads if reading fails part-way.
// ctx governs those downloads.
func NewResumingReader(ctx context.Context, downloader model.Downloader, address url.URL, body io.ReadCloser, size int64) io.ReadCloser {
	return newResumingReader(ctx, downloader, address, body, size)
}

func newResumingReader(ctx context.Context, downloader model.Downloader, address url.URL, body io.ReadCloser, size int64) *resumingReader {
	return &resumingReader{ctx: ctx, downloader: downloader, address: address, body: body, size: size}
}

func (this *resumingReader) Read(buffer []byte) (int, error) {
	for {
		count, err := this.body.Read(buffer)
		this.offset += int64(count)
		if err == nil || errors.Is(err, io.EOF) || !this.canResume() {
			return count, err
		}
		if resumeErr := this.resume(); resumeErr != nil {
			return count, errors.Join(err, resumeErr)
		}
		if count > 0 {
			return count, nil
		}
	}
}

func (this *resumingReader) canResume() bool {
	return this.size > 0 && this.offset < this.size && this.resumes < maxDownloadResumes && this.ctx.Err() == nil
}

func (this *resumingReader) resume() error {
	this.resumes++
	_ = this.body.Close()
	body, err := this.downloader.Seek(this.ctx, this.address, this.offset, this.size-1)
	if err != nil {
		return err
	}
	this.body = body
	return nil
}

func (this *resumingReader) Close() error {
	return this.body.Close()
}
