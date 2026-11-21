package packaging

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"syscall"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestResumingReaderFixture(t *testing.T) {
	gunit.Run(new(ResumingReaderFixture), t)
}

type ResumingReaderFixture struct {
	*gunit.Fixture
	content    []byte
	downloader *flakyDownloader
	address    url.URL
}

func (this *ResumingReaderFixture) Setup() {
	this.content = bytes.Repeat([]byte("0123456789"), 100)
	this.downloader = &flakyDownloader{content: this.content, failAfter: 250}
	this.address = url.URL{Scheme: "gcs", Host: "bucket", Path: "/archive"}
}

func (this *ResumingReaderFixture) open(size int64) *resumingReader {
	body, err := this.downloader.Download(context.Background(), this.address)
	this.So(err, should.BeNil)
	return newResumingReader(context.Background(), this.downloader, this.address, body, size)
}

func (this *ResumingReaderFixture) TestDroppedConnectionsResumeWhereTheyLeftOff() {
	reader := this.open(int64(len(this.content)))

	read, err := io.ReadAll(reader)

	this.So(err, should.BeNil)
	this.So(read, should.Resemble, this.content)
	this.So(this.downloader.ranges, should.Resemble, [][2]int64{{250, 999}, {500, 999}, {750, 999}})
}

func (this *ResumingReaderFixture) TestGivesUpAfterTooManyResumes() {
	this.downloader.failAfter = 10
	reader := this.open(int64(len(this.content)))

	_, err := io.ReadAll(reader)

	this.So(errors.Is(err, syscall.ECONNRESET), should.BeTrue)
	this.So(this.downloader.ranges, should.HaveLength, maxDownloadResumes)
}

func (this *ResumingReaderFixture) TestUnknownSizeDoesNotResume() {
	reader := this.open(0)

	_, err := io.ReadAll(reader)

	this.So(errors.Is(err, syscall.ECONNRESET), should.BeTrue)
	this.So(this.downloader.ranges, should.BeEmpty)
}

func (this *ResumingReaderFixture) TestFailedResumeIsReported() {
	resumeErr := errors.New("unavailable")
	this.downloader.seekErr = resumeErr
	reader := this.open(int64(len(this.content)))

	_, err := io.ReadAll(reader)

	this.So(errors.Is(err, syscall.ECONNRESET), should.BeTrue)
	this.So(errors.Is(err, resumeErr), should.BeTrue)
}

///////////////////////////////////////////////////////////////////////////////

// flakyDownloader serves content, but each response drops its connection
// after failAfter bytes.
type flakyDownloader struct {
	content   []byte
	failAfter int
	seekErr   error
	ranges    [][2]int64
}

func (this *flakyDownloader) Download(_ context.Context, _ url.URL) (io.ReadCloser, error) {
	return this.response(this.content), nil
}

func (this *flakyDownloader) Seek(_ context.Context, _ url.URL, start, end int64) (io.ReadCloser, error) {
	this.ranges = append(this.ranges, [2]int64{start, end})
	if this.seekErr != nil {
		return nil, this.seekErr
	}
	return this.response(this.content[start : end+1]), nil
}

func (this *flakyDownloader) Size(context.Context, url.URL) (int64, error) {
	return int64(len(this.content)), nil
}

func (this *flakyDownloader) response(content []byte) io.ReadCloser {
	if len(content) <= this.failAfter {
		return io.NopCloser(bytes.NewReader(content))
	}
	return io.NopCloser(io.MultiReader(
		bytes.NewReader(content[:this.failAfter]),
		errorReader{syscall.ECONNRESET},
	))
}

type errorReader struct{ err error }

func (this errorReader) Read([]byte) (int, error) { return 0, this.err }
