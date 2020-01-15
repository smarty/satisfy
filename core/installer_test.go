package core

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"testing"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/fs"
	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
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
	originalManifest := contracts.Manifest{Name: "Gordon", Version: "1.2.3"}
	raw, _ := json.Marshal(originalManifest)
	this.downloader.Body = ioutil.NopCloser(bytes.NewReader(raw))
	manifest, err := this.installer.InstallManifest(contracts.InstallationRequest{})
	this.So(manifest, should.Resemble, originalManifest)
	this.So(err, should.BeNil)
}

//err2 := this.installer.InstallPackage(manifest, contracts.InstallationRequest{LocalPath:""})

////////////////////////////////////////

type FakeDownloader struct {
	Body  io.ReadCloser
	Error error
}

func (this *FakeDownloader) Download(contracts.DownloadRequest) (io.ReadCloser, error) {
	return this.Body, this.Error
}
