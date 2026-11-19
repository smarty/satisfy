package satisfy

import (
	"errors"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestValidateDependenciesFixture(t *testing.T) {
	gunit.Run(new(ValidateDependenciesFixture), t)
}

type ValidateDependenciesFixture struct {
	*gunit.Fixture
}

func dependency(name, version, host, directory string) Dependency {
	return Dependency{
		PackageName:    name,
		PackageVersion: version,
		RemoteAddress:  URL{Scheme: "gcs", Host: host},
		LocalDirectory: directory,
	}
}

func (this *ValidateDependenciesFixture) assertInvalid(dependencies ...Dependency) {
	err := ValidateDependencies(dependencies...)
	this.So(errors.Is(err, ErrInvalidConfig), should.BeTrue)
}

func (this *ValidateDependenciesFixture) TestCompleteDependenciesAreValid() {
	this.So(ValidateDependencies(
		dependency("a", "1.2.3", "bucket", "local"),
		dependency("b", "1.2.3", "bucket", "local"),
		dependency("a", "1.2.3", "bucket", "elsewhere"),
	), should.BeNil)
}

func (this *ValidateDependenciesFixture) TestNoDependenciesAreValid() {
	this.So(ValidateDependencies(), should.BeNil)
}

func (this *ValidateDependenciesFixture) TestEachFieldIsRequired() {
	this.assertInvalid(dependency("", "1.2.3", "bucket", "local"))
	this.assertInvalid(dependency("name", "", "bucket", "local"))
	this.assertInvalid(dependency("name", "1.2.3", "", "local"))
	this.assertInvalid(dependency("name", "1.2.3", "bucket", ""))
}

func (this *ValidateDependenciesFixture) TestAPackageCannotBeInstalledTwiceIntoTheSameDirectory() {
	this.assertInvalid(
		dependency("name", "1.2.3", "bucket1", "local"),
		dependency("name", "4.5.6", "bucket2", "./local/"),
	)
}

func (this *ValidateDependenciesFixture) TestValidationDoesNotModifyItsInput() {
	dependencies := []Dependency{dependency("name", "1.2.3", "bucket", "~/path")}
	this.So(ValidateDependencies(dependencies...), should.BeNil)
	this.So(dependencies[0].LocalDirectory, should.Equal, "~/path")
}
