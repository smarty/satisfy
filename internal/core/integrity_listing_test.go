package core

import (
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/contracts"
)

func TestIntegrityListingFixture(t *testing.T) {
	gunit.Run(new(IntegrityListingFixture), t)
}

type IntegrityListingFixture struct {
	*gunit.Fixture

	checker    *FileListingIntegrityChecker
	fileSystem *inMemoryFileSystem
	manifest   contracts.Manifest
}

func (this *IntegrityListingFixture) Setup() {
	this.fileSystem = newInMemoryFileSystem()
	this.checker = NewFileListingIntegrityChecker(this.fileSystem)
	this.manifest = contracts.Manifest{
		Archive: contracts.Archive{
			Contents: []contracts.ArchiveItem{
				{Path: "/a", Size: 1},
				{Path: "/bb", Size: 2},
				{Path: "/cc/c", Size: 3},
				{Path: "/dddd", Size: 4},
			},
		},
	}
	this.fileSystem.WriteFile("/local/a", []byte("a"))
	this.fileSystem.WriteFile("/local/bb", []byte("bb"))
	this.fileSystem.WriteFile("/local/cc/c", []byte("ccc"))
	this.fileSystem.WriteFile("/local/dddd", []byte("dddd"))
}

func (this *IntegrityListingFixture) TestFileListingIntegrityCheck() {
	this.So(this.checker.Verify(this.manifest, "/local"), should.BeNil)
}

func (this *IntegrityListingFixture) TestManifestFileNotOnFileSystem() {
	this.manifest.Archive.Contents = append(this.manifest.Archive.Contents, contracts.ArchiveItem{
		Path: "/eeeee",
		Size: 5,
	})

	this.So(this.checker.Verify(this.manifest, "/local"), should.NotBeNil)
}

func (this *IntegrityListingFixture) TestFileSizeMismatch() {
	this.manifest.Archive.Contents[0].Size = 0

	this.So(this.checker.Verify(this.manifest, "/local"), should.NotBeNil)
}
