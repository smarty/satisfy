package cmd

import (
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

func (this *DependencyListingFixture) TestMultiplePackagesWithSameNameAndDifferentVersionButInSamePlace() {
	this.appendDependency("name", "1.2.3", "address", "local")
	this.appendDependency("name", "3.2.1", "address", "local")

	err := this.listing.Validate()

	this.So(err, should.NotBeNil)
}

func (this *DependencyListingFixture) appendDependency(name, version, address, directory string) {
	this.listing.Dependencies = append(this.listing.Dependencies, Dependency{
		PackageName:    name,
		PackageVersion: version,
		RemoteAddress: URL{
			Host: address,
		},
		LocalDirectory: directory,
	})
}
