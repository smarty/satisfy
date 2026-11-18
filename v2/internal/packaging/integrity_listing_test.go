package packaging

import (
	"fmt"
	"io/fs"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2/internal/model"
)

func TestIntegrityListingFixture(t *testing.T) {
	gunit.Run(new(IntegrityListingFixture), t)
}

type IntegrityListingFixture struct {
	*gunit.Fixture

	checker    *FileListingIntegrityChecker
	fileSystem *inMemoryFileSystem
	manifest   model.Manifest
}

func (this *IntegrityListingFixture) Setup() {
	this.fileSystem = newInMemoryFileSystem()
	this.checker = NewFileListingIntegrityChecker(this.fileSystem)
	this.manifest = model.Manifest{
		Archive: model.Archive{
			Contents: []model.ArchiveItem{
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
	this.manifest.Archive.Contents = append(this.manifest.Archive.Contents, model.ArchiveItem{
		Path: "/eeeee",
		Size: 5,
	})

	this.So(this.checker.Verify(this.manifest, "/local"), should.NotBeNil)
}

func (this *IntegrityListingFixture) TestFileSizeMismatch() {
	this.manifest.Archive.Contents[0].Size = 0

	this.So(this.checker.Verify(this.manifest, "/local"), should.NotBeNil)
}

func (this *IntegrityListingFixture) TestWrappedNotExistErrorIsReportedAsMissingFile() {
	checker := NewFileListingIntegrityChecker(wrappedNotExist{})

	err := checker.Verify(this.manifest, "/local")

	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.StartWith, "filename not found")
}

// wrappedNotExist reports every path as missing, wrapping fs.ErrNotExist as
// file systems commonly do.
type wrappedNotExist struct{}

func (wrappedNotExist) Stat(path string) (model.FileInfo, error) {
	return nil, fmt.Errorf("stat %s: %w", path, fs.ErrNotExist)
}
