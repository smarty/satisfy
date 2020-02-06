package core

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
	"github.com/smartystreets/logging"

	"bitbucket.org/smartystreets/satisfy/contracts"
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
	this.resolver = NewDependencyResolver(this.fileSystem, this.integrityChecker, this.packageInstaller, this.dependency)
	this.resolver.logger = logging.Capture()
	this.fileSystem.WriteFile("local/manifest_B|C.json", []byte("{}"))
}

func (this *DependencyResolverFixture) TestResolver() {
	manifest := contracts.Manifest{
		Name:    "B/C",
		Version: "D",
	}
	this.packageInstaller.remote = manifest

	err := this.resolver.Resolve()

	this.So(err, should.BeNil)
	this.assertNewPackageInstalled()
}

func (this *DependencyResolverFixture) TestManifestInstallationFailure() {
	manifestErr := errors.New("manifest failure")
	this.packageInstaller.installManifestErr = manifestErr

	err := this.resolver.Resolve()

	this.So(errors.Is(err, manifestErr), should.BeTrue)
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
}

func (this *DependencyResolverFixture) TestManifestPresentButMalformed() {
	this.fileSystem.WriteFile("local/manifest_B___C.json", []byte("malformed json"))

	err := this.resolver.Resolve()

	this.So(err, should.NotBeNil)
	this.So(this.packageInstaller.installManifestCounter, should.Equal, 0)
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
}

func (this *DependencyResolverFixture) TestLocalManifestHasWrongPackageName() {
	this.prepareLocalPackageAndManifest("not "+this.dependency.PackageName, this.dependency.PackageVersion)

	err := this.resolver.Resolve()

	this.So(err, should.BeNil)
	this.assertPreviouslyInstalledPackageUninstalled()
	this.assertNewPackageInstalled()
}

func (this *DependencyResolverFixture) TestLocalManifestHasWrongVersion() {
	this.prepareLocalPackageAndManifest(this.dependency.PackageName, "not"+this.dependency.PackageVersion)

	err := this.resolver.Resolve()

	this.So(err, should.BeNil)
	this.assertPreviouslyInstalledPackageUninstalled()
	this.assertNewPackageInstalled()
}

func (this *DependencyResolverFixture) TestIntegrityCheckFailure() {
	localManifest := this.prepareLocalPackageAndManifest(this.dependency.PackageName, this.dependency.PackageVersion)
	this.integrityChecker.err = errors.New("integrity check failure")

	err := this.resolver.Resolve()

	this.So(err, should.BeNil)
	this.assertPreviouslyInstalledPackageUninstalled()
	this.assertNewPackageInstalled()
	this.So(this.integrityChecker.localPath, should.Equal, this.dependency.LocalDirectory)
	this.So(this.integrityChecker.manifest, should.Resemble, localManifest)
}

func (this *DependencyResolverFixture) TestItsAllGood() {
	this.prepareLocalPackageAndManifest(this.dependency.PackageName, this.dependency.PackageVersion)

	err := this.resolver.Resolve()

	this.So(err, should.BeNil)
	this.So(this.fileSystem.fileSystem, should.ContainKey, "contents1")
	this.So(this.fileSystem.fileSystem, should.ContainKey, "contents2")
	this.So(this.fileSystem.fileSystem, should.ContainKey, "contents3")
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
	this.So(this.packageInstaller.installManifestCounter, should.Equal, 0)
}

func (this *DependencyResolverFixture) TestNoPreviousInstallation() {
	this.prepareLocalPackageAndManifest("bogus", "bogus")
	this.fileSystem.Delete("local/manifest_B___C.json")

	err := this.resolver.Resolve()

	this.So(err, should.BeNil)
	this.assertNewPackageInstalled()
	this.So(this.fileSystem.fileSystem, should.ContainKey, "contents1")
	this.So(this.fileSystem.fileSystem, should.ContainKey, "contents2")
	this.So(this.fileSystem.fileSystem, should.ContainKey, "contents3")
}

func (this *DependencyResolverFixture) TestFinalInstallationFailed() {
	installError := errors.New("install package error")
	this.packageInstaller.installPackageErr = installError

	err := this.resolver.Resolve()

	this.So(errors.Is(err, installError), should.BeTrue)
}

func (this *DependencyResolverFixture) assertNewPackageInstalled() {
	this.So(this.packageInstaller.installed, should.Resemble, this.packageInstaller.remote)
	this.So(this.packageInstaller.manifestRequest, should.Resemble, contracts.InstallationRequest{
		RemoteAddress: this.URL("gcs://A/B/C/D/manifest.json"),
		LocalPath:     "local",
	})
	this.So(this.packageInstaller.packageRequest, should.Resemble, contracts.InstallationRequest{
		RemoteAddress: this.URL("gcs://A/B/C/D/archive"),
		LocalPath:     "local",
	})
}

func (this *DependencyResolverFixture) prepareLocalPackageAndManifest(packageName string, packageVersion string) contracts.Manifest {
	manifest := contracts.Manifest{
		Name:    packageName,
		Version: packageVersion,
		Archive: contracts.Archive{
			Contents: []contracts.ArchiveItem{
				{Path: "contents1"},
				{Path: "contents2"},
				{Path: "contents3"},
			},
		},
	}
	raw, _ := json.Marshal(manifest)
	this.fileSystem.WriteFile("local/manifest_B___C.json", raw)
	this.fileSystem.WriteFile("contents1", []byte("contents1"))
	this.fileSystem.WriteFile("contents2", []byte("contents2"))
	this.fileSystem.WriteFile("contents3", []byte("contents3"))
	return manifest
}

func (this *DependencyResolverFixture) assertPreviouslyInstalledPackageUninstalled() {
	this.So(this.fileSystem.fileSystem, should.NotContainKey, "contents1")
	this.So(this.fileSystem.fileSystem, should.NotContainKey, "contents2")
	this.So(this.fileSystem.fileSystem, should.NotContainKey, "contents3")
}

func (this *DependencyResolverFixture) URL(address string) url.URL {
	parsed, err := url.Parse(address)
	this.So(err, should.BeNil)
	return *parsed
}

///////////////////////////////////////////////////////////////////////////////////////////////

type FakePackageInstaller struct {
	remote                 contracts.Manifest
	installed              contracts.Manifest
	manifestRequest        contracts.InstallationRequest
	packageRequest         contracts.InstallationRequest
	installManifestErr     error
	installPackageErr      error
	installManifestCounter int
	installPackageCounter  int
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
