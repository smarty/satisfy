package core

import (
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/fs"
)

func TestIntegrityListingFixture(t *testing.T) {
	gunit.Run(new(IntegrityListingFixture), t)
}

type IntegrityListingFixture struct {
	*gunit.Fixture

	checker    *FileListingIntegrityChecker
	fileSystem *fs.InMemoryFileSystem
	manifest   contracts.Manifest
}

func (this *IntegrityListingFixture) Setup() {
	this.fileSystem = fs.NewInMemoryFileSystem()
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
	this.fileSystem.WriteFile("/a", []byte("a"))
	this.fileSystem.WriteFile("/bb", []byte("bb"))
	this.fileSystem.WriteFile("/cc/c", []byte("ccc"))
	this.fileSystem.WriteFile("/dddd", []byte("dddd"))
}

func (this *IntegrityListingFixture) TestFileListingIntegrityCheck() {
	this.So(this.checker.Verify(this.manifest), should.BeNil)
}

func (this *IntegrityListingFixture) TestManifestFileNotOnFileSystem() {
	this.manifest.Archive.Contents = append(this.manifest.Archive.Contents, contracts.ArchiveItem{
		Path: "/eeeee",
		Size: 5,
	})

	this.So(this.checker.Verify(this.manifest), should.Resemble, errFileNotFound)
}

func (this *IntegrityListingFixture) TestFileSizeMismatch() {
	this.manifest.Archive.Contents[0].Size = 0

	this.So(this.checker.Verify(this.manifest), should.Resemble, errFileSizeMismatch)
}