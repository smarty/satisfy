package contracts

import (
	"net/url"
	"os"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
)

func TestDependencyListingFixture(t *testing.T) {
	gunit.Run(new(DependencyListingFixture), t)
}

type DependencyListingFixture struct {
	*gunit.Fixture

	listing DependencyListing
}

func (this *DependencyListingFixture) Setup() {
	this.listing = DependencyListing{}
}

func (this *DependencyListingFixture) TestValidateEachDependencyIsPopulated_NoError() {
	this.appendDependency("name", "1.2.3", "host", "directory")

	err := this.listing.Validate()

	this.So(err, should.BeNil)
}

func (this *DependencyListingFixture) TestValidateEachDependencyMustHaveALocalDirectory() {
	this.appendDependency("name", "1.2.3", "host", "")

	err := this.listing.Validate()

	this.So(err, should.NotBeNil)
}

func (this *DependencyListingFixture) TestValidateEachDependencyMustHaveAName() {
	this.appendDependency("", "1.2.3", "host", "local")

	err := this.listing.Validate()

	this.So(err, should.NotBeNil)
}

func (this *DependencyListingFixture) TestValidateEachDependencyMustHaveAVersion() {
	this.appendDependency("name", "", "host", "local")

	err := this.listing.Validate()

	this.So(err, should.NotBeNil)
}

func (this *DependencyListingFixture) TestValidateEachDependencyMustHaveARemoteAddress() {
	this.appendDependency("name", "1.2.3", "", "local")

	err := this.listing.Validate()

	this.So(err, should.NotBeNil)
}

func (this *DependencyListingFixture) TestValidateResolvesLocalDirectory() {
	this.appendDependency("name", "1.2.3", "address", "~/")
	this.appendDependency("name", "1.2.3", "address", "~/path1")
	this.appendDependency("name", "1.2.3", "address", "$HOME/path2")
	this.appendDependency("name", "1.2.3", "address", "${HOME}/path3")

	err := this.listing.Validate()
	home := os.Getenv("HOME")

	this.So(err, should.BeNil)
	this.So(this.listing.Listing, should.Resemble, []Dependency{
		{PackageName: "name", PackageVersion: "1.2.3", RemoteAddress: URL{Host: "address"}, LocalDirectory: home},
		{PackageName: "name", PackageVersion: "1.2.3", RemoteAddress: URL{Host: "address"}, LocalDirectory: home + "/path1"},
		{PackageName: "name", PackageVersion: "1.2.3", RemoteAddress: URL{Host: "address"}, LocalDirectory: home + "/path2"},
		{PackageName: "name", PackageVersion: "1.2.3", RemoteAddress: URL{Host: "address"}, LocalDirectory: home + "/path3"},
	})
}

func (this *DependencyListingFixture) TestMultiplePackagesWithSameNameCannotBeInstalledToTheSamePlace() {
	this.appendDependency("name", "1.2.3", "address1", "local")
	this.appendDependency("name", "1.2.3", "address2", "local")

	err := this.listing.Validate()

	this.So(err, should.NotBeNil)
}

func (this *DependencyListingFixture) TestAppendRemoteAddress() {
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

func (this *DependencyListingFixture) TestAppendRemoteAddressLatest() {
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

func (this *DependencyListingFixture) TestTitleString() {
	dependency := Dependency{
		PackageName:    "package-name",
		PackageVersion: "1.2.3",
	}

	this.So(dependency.Title(), should.Equal, "[package-name @ 1.2.3]")
}
func (this *DependencyListingFixture) appendDependency(name, version, address, directory string) {
	this.listing.Listing = append(this.listing.Listing, Dependency{
		PackageName:    name,
		PackageVersion: version,
		RemoteAddress: URL{
			Host: address,
		},
		LocalDirectory: directory,
	})
}
