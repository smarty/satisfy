package contracts

import (
	"net/url"
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

func (this *DependencyListingFixture) TestMultiplePackagesWithSameNameCannotBeInstalledToTheSamePlace() {
	this.appendDependency("name", "1.2.3", "address1", "local")
	this.appendDependency("name", "1.2.3", "address2", "local")

	err := this.listing.Validate()

	this.So(err, should.NotBeNil)
}

func (this *DependencyListingFixture) TestAppendRemoteAddress() {
	address, err := url.Parse("https://www.google.com")
	this.So(err, should.BeNil)
	dependency := Dependency{
		PackageName:    "package-name",
		PackageVersion: "1.2.3",
		RemoteAddress:  URL(*address),
	}
	actual := dependency.ComposeRemoteAddress("filename")

	this.So(actual.String(), should.Equal, "https://www.google.com/package-name/1.2.3/filename")
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
