package packaging

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/json/v2"
	"errors"
	"io"
	"log"
	"math/rand"
	"net/url"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2/internal/archive"
	"github.com/smarty/satisfy/v2/internal/model"
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
	this.installer = NewPackageInstaller(this.downloader, this.filesystem, nil, nil)
}

func (this *PackageInstallerFixture) TestInstallManifest() {
	originalManifest := model.Manifest{
		Name:    "Package/Name",
		Version: "1.2.3",
		// json/v2 writes nil slices as empty values, which read back as empty slices.
		Archive: model.Archive{MD5Checksum: []byte{}, Contents: []model.ArchiveItem{}},
	}
	this.downloader.prepareManifestDownload(originalManifest)

	request := this.installationRequest(originalManifest.Name)
	manifest, err := this.installer.InstallManifest(context.Background(), request)

	this.So(this.downloader.request, should.Resemble, request.RemoteAddress)
	this.So(manifest, should.Resemble, originalManifest)
	this.So(err, should.BeNil)
	fileName := "local/path/manifest_Package___Name.json"
	this.So(this.loadLocalManifest(fileName), should.Resemble, originalManifest)
}

func (this *PackageInstallerFixture) loadLocalManifest(fileName string) model.Manifest {
	reader, _ := this.filesystem.Open(fileName)
	var localManifest model.Manifest
	_ = json.UnmarshalRead(reader, &localManifest)
	return localManifest
}

func (this *PackageInstallerFixture) TestInstallManifestDownloadError() {
	downloadError := errors.New("something or other")
	this.downloader.Error = downloadError
	manifest, err := this.installer.InstallManifest(context.Background(), this.installationRequest(""))
	this.So(err, should.Equal, downloadError)
	this.So(manifest, should.BeZeroValue)
}

func (this *PackageInstallerFixture) TestInstallManifestJsonDecodingError() {
	this.downloader.prepareMalformedDownload()
	manifest, err := this.installer.InstallManifest(context.Background(), this.installationRequest(""))
	this.So(err, should.NotBeNil)
	this.So(manifest, should.BeZeroValue)
}

func (this *PackageInstallerFixture) TestDownloadManifestReadError() {
	readError := errors.New("connection reset")
	this.downloader.Body = io.NopCloser(iotest.ErrReader(readError))
	manifest, err := this.installer.DownloadManifest(context.Background(), this.installationRequest("").RemoteAddress)
	this.So(err, should.Equal, readError)
	this.So(manifest, should.BeZeroValue)
}

func (this *PackageInstallerFixture) TestInstallPackageToLocalFileSystemUsingGzipCompression() {
	checksum := this.downloader.prepareArchiveDownload(gzipAlgorithm)

	err := this.installer.InstallPackage(context.Background(), this.buildManifest(checksum, gzipAlgorithm), this.installationRequest(""))

	this.So(err, should.BeNil)
	this.So(this.filesystem.readFile("local/path/Hello/World"), should.Resemble, []byte("Hello World"))
	this.So(this.filesystem.readFile("local/path/Goodbye/World"), should.Resemble, []byte("Goodbye World"))
	this.So(this.filesystem.readFile("local/path/Link"), should.Resemble, []byte("Hello World"))
	this.So(this.filesystem.fileSystem["local/path/Goodbye/World"].Mode(), should.Equal, 0755)
}

func (this *PackageInstallerFixture) TestInstallPackageToLocalFileSystemUsingZstdCompression() {
	checksum := this.downloader.prepareArchiveDownload(zstdAlgorithm)

	err := this.installer.InstallPackage(context.Background(), this.buildManifest(checksum, zstdAlgorithm), this.installationRequest(""))

	this.So(err, should.BeNil)
	this.So(this.filesystem.readFile("local/path/Hello/World"), should.Resemble, []byte("Hello World"))
	this.So(this.filesystem.readFile("local/path/Goodbye/World"), should.Resemble, []byte("Goodbye World"))
	this.So(this.filesystem.readFile("local/path/Link"), should.Resemble, []byte("Hello World"))
}

func (this *PackageInstallerFixture) TestInstallPackageToLocalFileSystemUsingZipCompression() {
	checksum := this.downloader.prepareZipArchiveDownload()

	err := this.installer.InstallPackage(context.Background(), this.buildZipManifest(checksum), this.installationRequest(""))

	this.So(err, should.BeNil)
	this.So(this.filesystem.readFile("local/path/Hello/World"), should.Resemble, []byte("Hello World"))
	this.So(this.filesystem.readFile("local/path/Goodbye/World"), should.Resemble, []byte("Goodbye World"))
}

func (this *PackageInstallerFixture) TestInstallPackageZipChecksumMismatch() {
	this.downloader.prepareZipArchiveDownload()

	err := this.installer.InstallPackage(context.Background(), this.buildZipManifest([]byte("mismatch")), this.installationRequest(""))

	this.So(err, should.NotBeNil)
	this.So(this.filesystem.listing(), should.BeEmpty)
}

func (this *PackageInstallerFixture) TestInstallPackageMalformedZipArchive() {
	this.downloader.prepareMalformedDownload()

	err := this.installer.InstallPackage(context.Background(), this.buildZipManifest(nil), this.installationRequest(""))

	this.So(err, should.NotBeNil)
	this.So(this.filesystem.listing(), should.BeEmpty)
}

func (this *PackageInstallerFixture) TestCompressionMethodInvalid() {

	checksum := this.downloader.prepareArchiveDownload(gzipAlgorithm)

	err := this.installer.InstallPackage(context.Background(), this.buildManifest(checksum, "invalid"), this.installationRequest(""))

	this.So(err, should.NotBeNil)
}

func (this *PackageInstallerFixture) TestInstallPackageInvalidArchive() {
	this.downloader.prepareMalformedDownload()

	err := this.installer.InstallPackage(context.Background(), this.buildManifest(nil, gzipAlgorithm), this.installationRequest(""))

	this.So(err, should.NotBeNil)
	this.So(this.filesystem.listing(), should.BeEmpty)
}

func (this *PackageInstallerFixture) TestInstallPackageDownloadError() {
	this.downloader.Error = errors.New("i am an error")

	err := this.installer.InstallPackage(context.Background(), this.buildManifest(nil, gzipAlgorithm), this.installationRequest(""))

	this.So(err, should.NotBeNil)
	this.So(this.filesystem.listing(), should.BeEmpty)
}

func (this *PackageInstallerFixture) TestInstallPackageCreateErrorRevertsExtractedFiles() {
	checksum := this.downloader.prepareArchiveDownload(gzipAlgorithm)
	createErr := errors.New("disk full")
	this.filesystem.errCreate["local/path/Goodbye/World"] = createErr

	err := this.installer.InstallPackage(context.Background(), this.buildManifest(checksum, gzipAlgorithm), this.installationRequest(""))

	this.So(errors.Is(err, createErr), should.BeTrue)
	this.So(this.filesystem.listing(), should.BeEmpty)
}

func (this *PackageInstallerFixture) TestInstallPackageChecksumMismatch() {
	this.downloader.prepareArchiveDownload(gzipAlgorithm)

	err := this.installer.InstallPackage(context.Background(), this.buildManifest([]byte("mismatch"), gzipAlgorithm), this.installationRequest(""))

	this.So(err, should.NotBeNil)
	this.So(this.filesystem.listing(), should.BeEmpty)
}

// The tar reader stops at its end-of-archive marker, which can leave the end of
// the compressed stream (e.g. the gzip trailer) unread; the checksum must still
// cover every byte of the downloaded archive. These content sizes left the gzip
// trailer unread before the installer drained the stream.
func (this *PackageInstallerFixture) TestInstallPackageChecksumCoversEntireCompressedStream() {
	for _, algorithm := range []string{gzipAlgorithm, zstdAlgorithm} {
		for _, size := range []int{162183, 293101, 358560} {
			content := make([]byte, size)
			_, _ = rand.New(rand.NewSource(int64(size))).Read(content)
			for i := 0; i < len(content); i += 3 {
				content[i] = 'a' // partly compressible, so it is not stored uncompressed
			}
			checksum := this.downloader.prepareSingleFileArchiveDownload(algorithm, content)
			installer := NewPackageInstaller(this.downloader, newInMemoryFileSystem(), nil, nil)

			err := installer.InstallPackage(context.Background(), this.buildManifest(checksum, algorithm), this.installationRequest(""))

			if !this.So(err, should.BeNil) {
				this.Printf("algorithm: %s, size: %d", algorithm, size)
				return
			}
		}
	}
}

func (this *PackageInstallerFixture) TestInstallPackageRefusesEntryOutsideInstallationDirectory() {
	checksum := this.downloader.prepareTarArchiveDownload(
		&tar.Header{Name: "inside", Size: 1, Mode: 0644},
		&tar.Header{Name: "../outside", Size: 1, Mode: 0644},
	)

	err := this.installer.InstallPackage(context.Background(), this.buildManifest(checksum, gzipAlgorithm), this.installationRequest(""))

	this.So(err, should.NotBeNil)
	this.So(this.filesystem.listing(), should.BeEmpty) // "inside" was rolled back
}

func (this *PackageInstallerFixture) TestInstallPackageRefusesSymlinkPointingOutsideInstallationDirectory() {
	for _, linkname := range []string{"../outside", "/etc/passwd", "dir/../../outside"} {
		checksum := this.downloader.prepareTarArchiveDownload(
			&tar.Header{Typeflag: tar.TypeSymlink, Name: "link", Linkname: linkname},
		)

		err := this.installer.InstallPackage(context.Background(), this.buildManifest(checksum, gzipAlgorithm), this.installationRequest(""))

		this.So(err, should.NotBeNil)
		this.So(this.filesystem.listing(), should.BeEmpty)
	}
}

func (this *PackageInstallerFixture) buildManifest(checksum []byte, compressionAlgorithm string) model.Manifest {
	return model.Manifest{
		Archive: model.Archive{
			MD5Checksum: checksum,
			Contents: []model.ArchiveItem{
				{Path: "Hello/World"},
				{Path: "Goodbye/World"},
				{Path: "Link"},
			},
			CompressionAlgorithm: compressionAlgorithm,
		},
	}
}

// The zip format serves as both archive and compression, so there is no tar
// stream inside and symlinks cannot be represented.
func (this *PackageInstallerFixture) buildZipManifest(checksum []byte) model.Manifest {
	return model.Manifest{
		Archive: model.Archive{
			MD5Checksum: checksum,
			Contents: []model.ArchiveItem{
				{Path: "Hello/World"},
				{Path: "Goodbye/World"},
			},
			CompressionAlgorithm: zipAlgorithm,
		},
	}
}

func (this *PackageInstallerFixture) installationRequest(packageName string) model.InstallationRequest {
	return model.InstallationRequest{
		RemoteAddress: url.URL{Host: "bucket", Path: "resource"},
		LocalPath:     "local/path",
		PackageName:   packageName,
	}
}

///////////////////////////////////////////////////////////////////////////////////////////////

type FakeDownloader struct {
	Body    io.ReadCloser
	Error   error
	request url.URL
}

func (this *FakeDownloader) Download(_ context.Context, request url.URL) (io.ReadCloser, error) {
	this.request = request
	return this.Body, this.Error
}

func (this *FakeDownloader) Seek(_ context.Context, request url.URL, start, end int64) (io.ReadCloser, error) {
	this.request = request
	buff := bytes.NewBuffer([]byte{})
	_, err := io.Copy(buff, this.Body)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(buff.Bytes()[start:end])
	return io.NopCloser(reader), this.Error
}

func (this *FakeDownloader) Size(_ context.Context, request url.URL) (int64, error) {
	this.request = request
	buff := bytes.NewBuffer([]byte{})
	return io.Copy(buff, this.Body)
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

	this.Body = io.NopCloser(bytes.NewReader(writer.Bytes()))

	return hasher.Sum(nil)
}

func (this *FakeDownloader) prepareSingleFileArchiveDownload(compressionAlgorithm string, content []byte) []byte {
	hasher := md5.New()
	writer := bytes.NewBuffer(nil)
	compressor := compression[compressionAlgorithm](io.MultiWriter(hasher, writer), 4)
	archiveWriter := tar.NewWriter(compressor)
	_ = archiveWriter.WriteHeader(&tar.Header{Name: "file", Size: int64(len(content)), Mode: 0644})
	_, _ = archiveWriter.Write(content)
	_ = archiveWriter.Close()
	_ = compressor.Close()
	this.Body = io.NopCloser(bytes.NewReader(writer.Bytes()))
	return hasher.Sum(nil)
}

// prepareTarArchiveDownload writes each header (with one byte of content for
// regular files) into a gzip compressed tar archive.
func (this *FakeDownloader) prepareTarArchiveDownload(headers ...*tar.Header) []byte {
	hasher := md5.New()
	writer := bytes.NewBuffer(nil)
	compressor := compression[gzipAlgorithm](io.MultiWriter(hasher, writer), 4)
	archiveWriter := tar.NewWriter(compressor)
	for _, header := range headers {
		_ = archiveWriter.WriteHeader(header)
		if header.Typeflag != tar.TypeSymlink {
			_, _ = archiveWriter.Write([]byte("x"))
		}
	}
	_ = archiveWriter.Close()
	_ = compressor.Close()
	this.Body = io.NopCloser(bytes.NewReader(writer.Bytes()))
	return hasher.Sum(nil)
}

func (this *FakeDownloader) prepareZipArchiveDownload() []byte {
	hasher := md5.New()
	writer := bytes.NewBuffer(nil)
	archiveWriter := archive.NewZipArchiveWriter(io.MultiWriter(hasher, writer), 4)

	archiveWriter.WriteHeader(model.ArchiveHeader{
		Name:    "Hello/World",
		Size:    int64(len("Hello World")),
		ModTime: time.Now(),
	})
	_, _ = archiveWriter.Write([]byte("Hello World"))
	archiveWriter.WriteHeader(model.ArchiveHeader{
		Name:    "Goodbye/World",
		Size:    int64(len("Goodbye World")),
		ModTime: time.Now(),
	})
	_, _ = archiveWriter.Write([]byte("Goodbye World"))
	_ = archiveWriter.Close()

	this.Body = io.NopCloser(bytes.NewReader(writer.Bytes()))

	return hasher.Sum(nil)
}

func (this *FakeDownloader) prepareManifestDownload(manifest model.Manifest) {
	raw, _ := json.Marshal(manifest)
	this.Body = io.NopCloser(bytes.NewReader(raw))
}

func (this *FakeDownloader) prepareMalformedDownload() {
	this.Body = io.NopCloser(strings.NewReader("malformed"))
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
	zipAlgorithm  = "zip"
)
