package core

import (
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

func TestUninstallationFixture(t *testing.T) {
	gunit.Run(new(UninstallationFixture), t)
}

type UninstallationFixture struct {
	*gunit.Fixture
	deleted []string
}

func (this *UninstallationFixture) Test() {
	manifest := contracts.Manifest{
		Archive: contracts.Archive{
			Contents: []contracts.ArchiveItem{
				{Path: "a"},
				{Path: "b"},
				{Path: "c"},
				{Path: "d"},
			},
		},
	}
	Uninstall(manifest, this.delete)
	this.So(this.deleted, should.Resemble, []string{"a", "b", "c", "d"})
}

func (this *UninstallationFixture) delete(path string) {
	this.deleted = append(this.deleted, path)
}
