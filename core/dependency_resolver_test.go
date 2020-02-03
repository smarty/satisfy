package core

import (
	"net/url"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"

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

}

func (this *DependencyResolverFixture) TestResolver() {
	manifest := contracts.Manifest{
		Name:    "B/C",
		Version: "D",
	}
	this.packageInstaller.remote = manifest

	err := this.resolver.Resolve()

	this.So(err, should.BeNil)
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

func (this *DependencyResolverFixture) URL(address string) url.URL {
	parsed, err := url.Parse(address)
	this.So(err, should.BeNil)
	return *parsed
}

///////////////////////////////////////////////////////////////////////////////////////////////

type FakePackageInstaller struct {
	remote          contracts.Manifest
	installed       contracts.Manifest
	manifestRequest contracts.InstallationRequest
	packageRequest  contracts.InstallationRequest
}

func (this *FakePackageInstaller) InstallManifest(request contracts.InstallationRequest) (manifest contracts.Manifest, err error) {
	this.manifestRequest = request
	return this.remote, nil
}

func (this *FakePackageInstaller) InstallPackage(manifest contracts.Manifest, request contracts.InstallationRequest) {
	this.installed = manifest
	this.packageRequest = request
}
