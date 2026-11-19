package satisfy

import (
	"errors"
	"os"
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

func (this *ValidateDependenciesFixture) TestLocalDirectoriesAreResolvedAgainstHome() {
	home := os.Getenv("HOME")
	original := []Dependency{
		dependency("name", "1.2.3", "bucket", "~/"),
		dependency("name", "1.2.3", "bucket", "~/path1"),
		dependency("name", "1.2.3", "bucket", "$HOME/path2"),
		dependency("name", "1.2.3", "bucket", "${HOME}/path3"),
		dependency("name", "1.2.3", "bucket", "relative"),
	}

	resolved := resolveLocalDirectories(original)

	this.So(original[1].LocalDirectory, should.Equal, "~/path1")
	this.So(resolved[0].LocalDirectory, should.Equal, home)
	this.So(resolved[1].LocalDirectory, should.Equal, home+"/path1")
	this.So(resolved[2].LocalDirectory, should.Equal, home+"/path2")
	this.So(resolved[3].LocalDirectory, should.Equal, home+"/path3")
	this.So(resolved[4].LocalDirectory, should.Equal, "relative")
}

func (this *ValidateDependenciesFixture) TestHomeRelativeDuplicatesAreDetected() {
	this.assertInvalid(
		dependency("name", "1.2.3", "bucket", "~/path"),
		dependency("name", "1.2.3", "bucket", os.Getenv("HOME")+"/path"),
	)
}
