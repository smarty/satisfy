package transfer

import (
	"bytes"
	"crypto/md5"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/core"
	"github.com/smarty/satisfy/shell"
)

func TestUploadCompressionFixture(t *testing.T) {
	gunit.Run(new(UploadCompressionFixture), t)
}

type UploadCompressionFixture struct {
	*gunit.Fixture
	localPath string
}

func (this *UploadCompressionFixture) Setup() {
	localPath, err := os.MkdirTemp("", "satisfy-upload-test-*")
	this.So(err, should.BeNil)
	this.localPath = localPath
}

func (this *UploadCompressionFixture) Teardown() {
	_ = os.RemoveAll(this.localPath)
}

func (this *UploadCompressionFixture) TestGzipArchiveInstallsCorrectly() {
	this.assertArchiveRoundTrip("gzip", 4)
}

func (this *UploadCompressionFixture) TestZstdArchiveInstallsCorrectly() {
	this.assertArchiveRoundTrip("zstd", 4)
}

func (this *UploadCompressionFixture) TestZstdArchiveInstallsCorrectlyAtHighestSpeedLevel() {
	this.assertArchiveRoundTrip("zstd", 1)
}

func (this *UploadCompressionFixture) TestZipArchiveInstallsCorrectly() {
	this.assertArchiveRoundTrip("zip", 9)
}

func (this *UploadCompressionFixture) TestEveryCompressionAlgorithmHasAContentType() {
	this.So(len(contentType), should.Equal, len(compression))
	for algorithm := range compression {
		this.So(contentType[algorithm], should.NotBeBlank)
	}
}

// The write side mirrors UploadApp.buildArchiveAndManifestContents (archive
// closed by PackageBuilder.Build, then the compressor closed by the app); the
// read side mirrors DownloadApp's installer wiring. Zip cannot represent
// symlinks or executable modes, so those entries are asserted only for the
// tar-based algorithms.
func (this *UploadCompressionFixture) assertArchiveRoundTrip(algorithm string, level int) {
	payload := archivePayload()
	hasher := md5.New()
	buffer := bytes.NewBuffer(nil)
	compressor := compression[algorithm](io.MultiWriter(hasher, buffer), level)
	archive := shell.NewSwitchArchiveWriter(compressor)

	archive.WriteHeader(contracts.ArchiveHeader{
		Name:    "Hello/World",
		Size:    int64(len("Hello World")),
		ModTime: archiveModTime,
	})
	_, err := archive.Write([]byte("Hello World"))
	this.So(err, should.BeNil)
	archive.WriteHeader(contracts.ArchiveHeader{
		Name:       "bin/tool",
		Size:       int64(len(payload)),
		ModTime:    archiveModTime,
		Executable: true,
	})
	_, err = archive.Write(payload)
	this.So(err, should.BeNil)
	if algorithm != "zip" {
		archive.WriteHeader(contracts.ArchiveHeader{
			Name:     "Link",
			LinkName: "Hello/World",
			ModTime:  archiveModTime,
		})
	}
	this.So(archive.Close(), should.BeNil)
	this.So(compressor.Close(), should.BeNil)

	installer := core.NewPackageInstaller(
		&fakeArchiveDownloader{body: buffer.Bytes()},
		shell.NewDiskFileSystem(""),
		false,
	)
	manifest := contracts.Manifest{
		Archive: contracts.Archive{
			MD5Checksum: hasher.Sum(nil),
			Contents: []contracts.ArchiveItem{
				{Path: "Hello/World"},
				{Path: "bin/tool"},
			},
			CompressionAlgorithm: algorithm,
		},
	}
	err = installer.InstallPackage(manifest, contracts.InstallationRequest{
		RemoteAddress: url.URL{Host: "bucket", Path: "resource"},
		LocalPath:     this.localPath,
	})
	this.So(err, should.BeNil)

	helloContents, err := os.ReadFile(filepath.Join(this.localPath, "Hello/World"))
	this.So(err, should.BeNil)
	this.So(string(helloContents), should.Equal, "Hello World")

	toolContents, err := os.ReadFile(filepath.Join(this.localPath, "bin/tool"))
	this.So(err, should.BeNil)
	this.So(bytes.Equal(toolContents, payload), should.BeTrue)

	if algorithm != "zip" {
		toolInfo, err := os.Stat(filepath.Join(this.localPath, "bin/tool"))
		this.So(err, should.BeNil)
		this.So(toolInfo.Mode()&0111 != 0, should.BeTrue)

		linkInfo, err := os.Lstat(filepath.Join(this.localPath, "Link"))
		this.So(err, should.BeNil)
		this.So(linkInfo.Mode()&os.ModeSymlink != 0, should.BeTrue)

		linkContents, err := os.ReadFile(filepath.Join(this.localPath, "Link"))
		this.So(err, should.BeNil)
		this.So(string(linkContents), should.Equal, "Hello World")
	}
}

func archivePayload() []byte {
	payload := make([]byte, 32*1024)
	for i := range payload {
		payload[i] = byte(i * 7 % 253)
	}
	return payload
}

var archiveModTime = time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

type fakeArchiveDownloader struct {
	body []byte
}

func (this *fakeArchiveDownloader) Download(url.URL) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(this.body)), nil
}

func (this *fakeArchiveDownloader) Seek(_ url.URL, start, end int64) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(this.body[start:end])), nil
}

func (this *fakeArchiveDownloader) Size(url.URL) (int64, error) {
	return int64(len(this.body)), nil
}
