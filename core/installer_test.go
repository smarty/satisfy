package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/fs"
)

func TestPackageInstallerFixture(t *testing.T) {
	gunit.Run(new(PackageInstallerFixture), t)
}

type PackageInstallerFixture struct {
	*gunit.Fixture
	installer  *PackageInstaller
	downloader *FakeDownloader
	filesystem *fs.InMemoryFileSystem
}

func (this *PackageInstallerFixture) Setup() {
	this.downloader = &FakeDownloader{}
	this.filesystem = fs.NewInMemoryFileSystem()
	this.installer = NewPackageInstaller(this.downloader, this.filesystem)
}

func (this *PackageInstallerFixture) TestInstallManifest() {
	originalManifest := contracts.Manifest{Name: "Gor/don", Version: "1.2.3"}
	this.downloader.prepareManifestDownload(originalManifest)
	request := contracts.InstallationRequest{
		DownloadRequest: contracts.DownloadRequest{
			Bucket:   "bucket",
			Resource: "resource",
		},
		LocalPath: "local/path",
	}

	manifest, err := this.installer.InstallManifest(request)

	this.So(this.downloader.request, should.Resemble, contracts.DownloadRequest{
		Bucket:   "bucket",
		Resource: "resource",
	})
	this.So(manifest, should.Resemble, originalManifest)
	this.So(err, should.BeNil)
	reader := this.filesystem.Open("local/path/manifest_Gor|don_1.2.3.json")
	decoder := json.NewDecoder(reader)
	var localManifest contracts.Manifest
	decoder.Decode(&localManifest)
	this.So(localManifest, should.Resemble, originalManifest)
}

func (this *PackageInstallerFixture) TestInstallManifestDownloadError() {
	downloadError := errors.New("something or other")
	this.downloader.Error = downloadError
	manifest, err := this.installer.InstallManifest(contracts.InstallationRequest{})
	this.So(err, should.Equal, downloadError)
	this.So(manifest, should.BeZeroValue)
}

func (this *PackageInstallerFixture) TestInstallManifestJsonDecodingError() {
	this.downloader.prepareMalformedManifestDownload()
	manifest, err := this.installer.InstallManifest(contracts.InstallationRequest{})
	this.So(err, should.NotBeNil)
	this.So(manifest, should.BeZeroValue)
}

//err2 := this.installer.InstallPackage(manifest, contracts.InstallationRequest{LocalPath:""})

////////////////////////////////////////

type FakeDownloader struct {
	Body    io.ReadCloser
	Error   error
	request contracts.DownloadRequest
}

func (this *FakeDownloader) Download(request contracts.DownloadRequest) (io.ReadCloser, error) {
	this.request = request
	return this.Body, this.Error
}

func (this *FakeDownloader) prepareManifestDownload(manifest contracts.Manifest) {
	raw, _ := json.Marshal(manifest)
	this.Body = ioutil.NopCloser(bytes.NewReader(raw))
}

func (this *FakeDownloader) prepareMalformedManifestDownload() {
	this.Body = ioutil.NopCloser(strings.NewReader("malformed json"))
}
