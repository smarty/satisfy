package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
	"github.com/smartystreets/satisfy/contracts"
)

func TestDependencyResolverFixture(t *testing.T) {
	gunit.Run(new(DependencyResolverFixture), t)
}

type DependencyResolverFixture struct {
	*gunit.Fixture
	resolver         *DependencyResolver
	fileSystem       *inMemoryFileSystem
	integrityChecker *FakeIntegrityCheck
	packageInstaller *FakePackageInstaller
	dependency       contracts.Dependency
}

func (this *DependencyResolverFixture) Setup() {
	this.integrityChecker = &FakeIntegrityCheck{}
	this.fileSystem = newInMemoryFileSystem()
	this.packageInstaller = &FakePackageInstaller{}
	this.dependency = contracts.Dependency{
		PackageName:    "B/C",
		PackageVersion: "D",
		RemoteAddress:  contracts.URL(this.URL("gcs://A")),
		LocalDirectory: "local",
	}
	this.fileSystem.WriteFile("local/manifest_B|C.json", []byte("{}"))
}

func (this *DependencyResolverFixture) Resolve() error {
	this.resolver = NewDependencyResolver(this.fileSystem, this.integrityChecker, this.packageInstaller, this.dependency)
	return this.resolver.Resolve()
}

func (this *DependencyResolverFixture) TestFreshInstallation() {
	manifest := contracts.Manifest{
		Name:    "B/C",
		Version: "D",
		Archive: contracts.Archive{Filename: "archive-name"},
	}
	this.packageInstaller.remote = manifest

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.assertNewPackageInstalled(this.dependency.PackageVersion)
}

func (this *DependencyResolverFixture) TestManifestInstallationFailure() {
	manifestErr := errors.New("manifest failure")
	this.packageInstaller.installManifestErr = manifestErr

	err := this.Resolve()

	this.So(errors.Is(err, manifestErr), should.BeTrue)
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
}

func (this *DependencyResolverFixture) TestManifestFileCannotBeRead() {
	readFileErr := errors.New("manifest file cannot be read")
	this.fileSystem.WriteFile("local/manifest_B___C.json", []byte("malformed json"))
	this.fileSystem.errReadFile["local/manifest_B___C.json"] = readFileErr

	err := this.Resolve()

	this.So(err, should.Resemble, readFileErr)
	this.So(this.packageInstaller.installManifestCounter, should.Equal, 0)
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
}

func (this *DependencyResolverFixture) TestManifestPresentButMalformed() {
	this.fileSystem.WriteFile("local/manifest_B___C.json", []byte("malformed json"))

	err := this.Resolve()

	this.So(err, should.NotBeNil)
	this.So(this.packageInstaller.installManifestCounter, should.Equal, 0)
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
}

func (this *DependencyResolverFixture) TestLocalManifestHasWrongPackageName() {
	this.prepareLocalPackageAndManifest("not "+this.dependency.PackageName, this.dependency.PackageVersion)

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.assertPreviouslyInstalledPackageUninstalled()
	this.assertNewPackageInstalled(this.dependency.PackageVersion)
}

func (this *DependencyResolverFixture) TestLocalManifestHasWrongVersion() {
	this.prepareLocalPackageAndManifest(this.dependency.PackageName, "not"+this.dependency.PackageVersion)

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.assertPreviouslyInstalledPackageUninstalled()
	this.assertNewPackageInstalled(this.dependency.PackageVersion)
}

func (this *DependencyResolverFixture) TestIntegrityCheckFailure() {
	localManifest := this.prepareLocalPackageAndManifest(this.dependency.PackageName, this.dependency.PackageVersion)
	this.integrityChecker.err = errors.New("integrity check failure")

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.assertPreviouslyInstalledPackageUninstalled()
	this.assertNewPackageInstalled(this.dependency.PackageVersion)
	this.So(this.integrityChecker.localPath, should.Equal, this.dependency.LocalDirectory)
	this.So(this.integrityChecker.manifest, should.Resemble, localManifest)
}

func (this *DependencyResolverFixture) TestAlreadyInstalledCorrectly() {
	this.prepareLocalPackageAndManifest(this.dependency.PackageName, this.dependency.PackageVersion)

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents1")
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents2")
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents3")
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
	this.So(this.packageInstaller.installManifestCounter, should.Equal, 0)
}

func (this *DependencyResolverFixture) TestFinalInstallationFailed() {
	installError := errors.New("install package error")
	this.packageInstaller.installPackageErr = installError

	err := this.Resolve()

	this.So(errors.Is(err, installError), should.BeTrue)
}

func (this *DependencyResolverFixture) TestLatestIsAlreadyInstalled() {
	manifest := contracts.Manifest{
		Name:    "B/C",
		Version: "D",
	}
	this.packageInstaller.remoteLatest = manifest

	this.prepareLocalPackageAndManifest(this.dependency.PackageName, this.dependency.PackageVersion)
	this.dependency.PackageVersion = "latest"

	err := this.Resolve()

	this.So(err, should.BeNil)
}

func (this *DependencyResolverFixture) TestLocalPackageIsBehindLatest() {
	this.packageInstaller.remote = contracts.Manifest{
		Name:    "B/C",
		Version: "D",
	}

	this.packageInstaller.remoteLatest = contracts.Manifest{
		Name:    "B/C",
		Version: "E",
	}

	this.prepareLocalPackageAndManifest(this.dependency.PackageName, this.dependency.PackageVersion)
	this.dependency.PackageVersion = "latest"

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.assertPreviouslyInstalledPackageUninstalled()
	this.assertNewPackageInstalled("E")
}

func (this *DependencyResolverFixture) TestLatestFreshInstallation() {
	manifest := contracts.Manifest{
		Name:    "B/C",
		Version: "D",
		Archive: contracts.Archive{Filename: "archive-name"},
	}
	this.packageInstaller.remote = manifest
	this.dependency.PackageVersion = "latest"
	version := manifest.Version

	err := this.Resolve()

	this.assertLatestPackageInstalled(err, version)
}

func (this *DependencyResolverFixture) assertLatestPackageInstalled(err error, version string) {
	this.So(err, should.BeNil)
	this.So(this.packageInstaller.installed, should.Resemble, this.packageInstaller.remote)
	this.So(this.packageInstaller.manifestRequest, should.Resemble, contracts.InstallationRequest{
		RemoteAddress: this.URL("gcs://A/B/C/manifest.json"),
		LocalPath:     "local",
	})
	this.So(this.packageInstaller.packageRequest, should.Resemble, contracts.InstallationRequest{
		RemoteAddress: this.URL(fmt.Sprintf("gcs://A/B/C/%s/archive", version)),
		LocalPath:     "local",
	})
}
func (this *DependencyResolverFixture) TestLatestManifestFailsToDownload() {
	this.prepareLocalPackageAndManifest(this.dependency.PackageName, this.dependency.PackageVersion)
	this.dependency.PackageVersion = "latest"

	this.packageInstaller.downloadError = errors.New("error")
	this.packageInstaller.installManifestErr = errors.New("error")

	err := this.Resolve()

	this.So(err, should.NotBeNil)
	this.assertPreviouslyInstalledPackageUninstalled()
	this.So(this.fileSystem.fileSystem, should.NotContainKey, "local/contents1")
	this.So(this.fileSystem.fileSystem, should.NotContainKey, "local/contents2")
	this.So(this.fileSystem.fileSystem, should.NotContainKey, "local/contents3")
}

func (this *DependencyResolverFixture) assertNewPackageInstalled(version string) {
	this.So(this.packageInstaller.installed, should.Resemble, this.packageInstaller.remote)
	this.So(this.packageInstaller.manifestRequest, should.Resemble, contracts.InstallationRequest{
		RemoteAddress: this.URL(fmt.Sprintf("gcs://A/B/C/%s/manifest.json", version)),
		LocalPath:     "local",
	})
	this.So(this.packageInstaller.packageRequest, should.Resemble, contracts.InstallationRequest{
		RemoteAddress: this.URL(fmt.Sprintf("gcs://A/B/C/%s/archive", version)),
		LocalPath:     "local",
	})
}

func (this *DependencyResolverFixture) prepareLocalPackageAndManifest(
	packageName string, packageVersion string,
) contracts.Manifest {
	manifest := contracts.Manifest{
		Name:    packageName,
		Version: packageVersion,
		Archive: contracts.Archive{
			Filename: "archive",
			Contents: []contracts.ArchiveItem{
				{Path: "contents1"},
				{Path: "contents2"},
				{Path: "contents3"},
			},
		},
	}
	raw, _ := json.Marshal(manifest)
	this.fileSystem.WriteFile("local/manifest_B___C.json", raw)
	this.fileSystem.WriteFile("local/contents1", []byte("contents1"))
	this.fileSystem.WriteFile("local/contents2", []byte("contents2"))
	this.fileSystem.WriteFile("local/contents3", []byte("contents3"))
	return manifest
}

func (this *DependencyResolverFixture) assertPreviouslyInstalledPackageUninstalled() {
	this.So(this.fileSystem.fileSystem, should.NotContainKey, "local/contents1")
	this.So(this.fileSystem.fileSystem, should.NotContainKey, "local/contents2")
	this.So(this.fileSystem.fileSystem, should.NotContainKey, "local/contents3")
}

func (this *DependencyResolverFixture) URL(address string) url.URL {
	parsed, err := url.Parse(address)
	this.So(err, should.BeNil)
	return *parsed
}

///////////////////////////////////////////////////////////////////////////////////////////////

type FakePackageInstaller struct {
	remote                 contracts.Manifest
	remoteLatest           contracts.Manifest
	installed              contracts.Manifest
	manifestRequest        contracts.InstallationRequest
	packageRequest         contracts.InstallationRequest
	installManifestErr     error
	installPackageErr      error
	installManifestCounter int
	installPackageCounter  int
	downloadError          error
}

func (this *FakePackageInstaller) DownloadManifest(remoteAddress url.URL) (manifest contracts.Manifest, err error) {
	return this.remoteLatest, this.downloadError
}

func (this *FakePackageInstaller) InstallManifest(request contracts.InstallationRequest) (manifest contracts.Manifest, err error) {
	this.installManifestCounter++
	this.manifestRequest = request
	return this.remote, this.installManifestErr
}

func (this *FakePackageInstaller) InstallPackage(manifest contracts.Manifest, request contracts.InstallationRequest) error {
	this.installPackageCounter++
	this.installed = manifest
	this.packageRequest = request
	return this.installPackageErr
}
