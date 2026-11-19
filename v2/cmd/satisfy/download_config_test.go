package main

import (
	"os"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2"
)

func TestLocalDirectoryFixture(t *testing.T) {
	gunit.Run(new(LocalDirectoryFixture), t)
}

type LocalDirectoryFixture struct {
	*gunit.Fixture
}

func (this *LocalDirectoryFixture) TestLocalDirectoriesAreResolvedAgainstHome() {
	home := os.Getenv("HOME")
	original := []satisfy.Dependency{
		{LocalDirectory: "~/"},
		{LocalDirectory: "~/path1"},
		{LocalDirectory: "$HOME/path2"},
		{LocalDirectory: "${HOME}/path3"},
		{LocalDirectory: "relative"},
	}

	resolved := resolveLocalDirectories(original)

	this.So(original[1].LocalDirectory, should.Equal, "~/path1")
	this.So(resolved[0].LocalDirectory, should.Equal, home)
	this.So(resolved[1].LocalDirectory, should.Equal, home+"/path1")
	this.So(resolved[2].LocalDirectory, should.Equal, home+"/path2")
	this.So(resolved[3].LocalDirectory, should.Equal, home+"/path3")
	this.So(resolved[4].LocalDirectory, should.Equal, "relative")
}
