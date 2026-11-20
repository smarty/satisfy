package model

import (
	"net/url"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestDependencyFixture(t *testing.T) {
	gunit.Run(new(DependencyFixture), t)
}

type DependencyFixture struct {
	*gunit.Fixture
}

func (this *DependencyFixture) TestAppendRemoteAddress() {
	address, err := url.Parse("https://www.google.com/folder")
	this.So(err, should.BeNil)
	dependency := Dependency{
		PackageName:    "package-name",
		PackageVersion: "1.2.3",
		RemoteAddress:  URL(*address),
	}
	actual := dependency.ComposeRemoteAddress("filename")

	this.So(actual.String(), should.Equal, "https://www.google.com/folder/package-name/1.2.3/filename")
}

func (this *DependencyFixture) TestAppendRemoteAddressLatest() {
	address, err := url.Parse("https://www.google.com/folder")
	this.So(err, should.BeNil)
	dependency := Dependency{
		PackageName:    "package-name",
		PackageVersion: "latest",
		RemoteAddress:  URL(*address),
	}
	actual := dependency.ComposeRemoteAddress("manifest")

	this.So(actual.String(), should.Equal, "https://www.google.com/folder/package-name/manifest")
}

func (this *DependencyFixture) TestComposeLatestManifestRemoteAddress() {
	dependency := Dependency{
		PackageName:   "master-address-list/2026/04/premium/az",
		RemoteAddress: URL{Scheme: "gcs", Host: "my-bucket"},
	}
	address := dependency.ComposeLatestManifestRemoteAddress()
	this.So(address, should.Resemble, url.URL{
		Scheme: "gcs",
		Host:   "my-bucket",
		Path:   "/master-address-list/2026/04/premium/az/manifest.json",
	})
}

func (this *DependencyFixture) TestComposeLatestManifestRemoteAddressWithPathPrefix() {
	dependency := Dependency{
		PackageName:   "master-address-list/2026/04/premium/az",
		RemoteAddress: URL{Scheme: "gcs", Host: "my-bucket", Path: "/releases"},
	}
	address := dependency.ComposeLatestManifestRemoteAddress()
	this.So(address, should.Resemble, url.URL{
		Scheme: "gcs",
		Host:   "my-bucket",
		Path:   "/releases/master-address-list/2026/04/premium/az/manifest.json",
	})
}

func (this *DependencyFixture) TestTitleString() {
	dependency := Dependency{
		PackageName:    "package-name",
		PackageVersion: "1.2.3",
	}

	this.So(dependency.Title(), should.Equal, "[package-name @ 1.2.3]")
}
