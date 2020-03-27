package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
	"github.com/smartystreets/logging"
	"github.com/smartystreets/satisfy/contracts"
)

func TestPackageInstallerFixture(t *testing.T) {
	gunit.Run(new(PackageInstallerFixture), t)
}

type PackageInstallerFixture struct {
	*gunit.Fixture
	installer  *PackageInstaller
	downloader *FakeDownloader
	filesystem *inMemoryFileSystem
}

func (this *PackageInstallerFixture) Setup() {
	this.downloader = &FakeDownloader{}
	this.filesystem = newInMemoryFileSystem()
	this.installer = NewPackageInstaller(this.downloader, this.filesystem)
	this.installer.logger = logging.Capture()
}

func (this *PackageInstallerFixture) TestInstallManifest() {
	originalManifest := contracts.Manifest{Name: "Package/Name", Version: "1.2.3"}
	this.downloader.prepareManifestDownload(originalManifest)

	manifest, err := this.installer.InstallManifest(this.installationRequest())

	this.So(this.downloader.request, should.Resemble, this.installationRequest().RemoteAddress)
	this.So(manifest, should.Resemble, originalManifest)
	this.So(err, should.BeNil)
	fileName := "local/path/manifest_Package___Name.json"
	this.So(this.loadLocalManifest(fileName), should.Resemble, originalManifest)
}

func (this *PackageInstallerFixture) loadLocalManifest(fileName string) contracts.Manifest {
	reader := this.filesystem.Open(fileName)
	decoder := json.NewDecoder(reader)
	var localManifest contracts.Manifest
	_ = decoder.Decode(&localManifest)
	return localManifest
}

func (this *PackageInstallerFixture) TestInstallManifestDownloadError() {
	downloadError := errors.New("something or other")
	this.downloader.Error = downloadError
	manifest, err := this.installer.InstallManifest(this.installationRequest())
	this.So(err, should.Equal, downloadError)
	this.So(manifest, should.BeZeroValue)
}

func (this *PackageInstallerFixture) TestInstallManifestJsonDecodingError() {
	this.downloader.prepareMalformedDownload()
	manifest, err := this.installer.InstallManifest(this.installationRequest())
	this.So(err, should.NotBeNil)
	this.So(manifest, should.BeZeroValue)
}

func (this *PackageInstallerFixture) TestInstallPackageToLocalFileSystemUsingGzipCompression() {
	checksum := this.downloader.prepareArchiveDownload(gzipAlgorithm)

	err := this.installer.InstallPackage(this.buildManifest(checksum, gzipAlgorithm), this.installationRequest())

	this.So(err, should.BeNil)
	this.So(this.filesystem.readFile("local/path/Hello/World"), should.Resemble, []byte("Hello World"))
	this.So(this.filesystem.readFile("local/path/Goodbye/World"), should.Resemble, []byte("Goodbye World"))
	this.So(this.filesystem.readFile("local/path/Link"), should.Resemble, []byte("Hello World"))
	this.So(this.filesystem.fileSystem["local/path/Goodbye/World"].Mode(), should.Equal, 0755)
}

func (this *PackageInstallerFixture) LongTestInstallPackageToLocalFileSystemUsingZstdCompression() {
	checksum := this.downloader.prepareArchiveDownload(zstdAlgorithm)

	err := this.installer.InstallPackage(this.buildManifest(checksum, zstdAlgorithm), this.installationRequest())

	this.So(err, should.BeNil)
	this.So(this.filesystem.readFile("local/path/Hello/World"), should.Resemble, []byte("Hello World"))
	this.So(this.filesystem.readFile("local/path/Goodbye/World"), should.Resemble, []byte("Goodbye World"))
	this.So(this.filesystem.readFile("local/path/Link"), should.Resemble, []byte("Hello World"))
}

func (this *PackageInstallerFixture) TestCompressionMethodInvalid() {

	checksum := this.downloader.prepareArchiveDownload(gzipAlgorithm)

	err := this.installer.InstallPackage(this.buildManifest(checksum, "invalid"), this.installationRequest())

	this.So(err, should.NotBeNil)
}

func (this *PackageInstallerFixture) TestInstallPackageInvalidArchive() {
	this.downloader.prepareMalformedDownload()

	err := this.installer.InstallPackage(this.buildManifest(nil, gzipAlgorithm), this.installationRequest())

	this.So(err, should.NotBeNil)
	this.So(this.filesystem.Listing(), should.BeEmpty)
}

func (this *PackageInstallerFixture) TestInstallPackageDownloadError() {
	this.downloader.Error = errors.New("i am an error")

	err := this.installer.InstallPackage(this.buildManifest(nil, gzipAlgorithm), this.installationRequest())

	this.So(err, should.NotBeNil)
	this.So(this.filesystem.Listing(), should.BeEmpty)
}

func (this *PackageInstallerFixture) TestInstallPackageChecksumMismatch() {
	this.downloader.prepareArchiveDownload(gzipAlgorithm)

	err := this.installer.InstallPackage(this.buildManifest([]byte("mismatch"), gzipAlgorithm), this.installationRequest())

	this.So(err, should.NotBeNil)
	this.So(this.filesystem.Listing(), should.BeEmpty)
}

func (this *PackageInstallerFixture) buildManifest(checksum []byte, compressionAlgorithm string) contracts.Manifest {
	return contracts.Manifest{
		Archive: contracts.Archive{
			MD5Checksum: checksum,
			Contents: []contracts.ArchiveItem{
				{Path: "Hello/World"},
				{Path: "Goodbye/World"},
				{Path: "Link"},
			},
			CompressionAlgorithm: compressionAlgorithm,
		},
	}
}

func (this *PackageInstallerFixture) installationRequest() contracts.InstallationRequest {
	return contracts.InstallationRequest{
		RemoteAddress: url.URL{Host: "bucket", Path: "resource"},
		LocalPath:     "local/path",
	}
}

///////////////////////////////////////////////////////////////////////////////////////////////

type FakeDownloader struct {
	Body    io.ReadCloser
	Error   error
	request url.URL
}

func (this *FakeDownloader) Download(request url.URL) (io.ReadCloser, error) {
	this.request = request
	return this.Body, this.Error
}

func (this *FakeDownloader) prepareArchiveDownload(compressionAlgorithm string) []byte {
	hasher := md5.New()
	writer := bytes.NewBuffer(nil)
	multi := io.MultiWriter(hasher, writer)
	compressor := compression[compressionAlgorithm](multi, 4)
	archiveWriter := tar.NewWriter(compressor)

	_ = archiveWriter.WriteHeader(&tar.Header{
		Name: "Hello/World",
		Size: int64(len("Hello World")),
	})
	_, _ = archiveWriter.Write([]byte("Hello World"))
	_ = archiveWriter.WriteHeader(&tar.Header{
		Name: "Goodbye/World",
		Size: int64(len("Goodbye World")),
		Mode: 0755,
	})
	_, _ = archiveWriter.Write([]byte("Goodbye World"))
	_ = archiveWriter.WriteHeader(&tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     "Link",
		Linkname: "Hello/World",
	})
	_ = archiveWriter.Close()
	_ = compressor.Close()

	this.Body = ioutil.NopCloser(bytes.NewReader(writer.Bytes()))

	return hasher.Sum(nil)
}

func (this *FakeDownloader) prepareManifestDownload(manifest contracts.Manifest) {
	raw, _ := json.Marshal(manifest)
	this.Body = ioutil.NopCloser(bytes.NewReader(raw))
}

func (this *FakeDownloader) prepareMalformedDownload() {
	this.Body = ioutil.NopCloser(strings.NewReader("malformed"))
}

var compression = map[string]func(_ io.Writer, level int) io.WriteCloser{
	"zstd": func(writer io.Writer, level int) io.WriteCloser {
		compressor, err := zstd.NewWriter(writer, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)))
		if err != nil {
			log.Fatal(err)
		}
		return compressor
	},
	gzipAlgorithm: func(writer io.Writer, level int) io.WriteCloser {
		compressor, err := gzip.NewWriterLevel(writer, level)
		if err != nil {
			log.Panicln(err)
		}
		return compressor
	},
}

const (
	gzipAlgorithm = "gzip"
	zstdAlgorithm = "zstd"
)
