package core

import (
	"testing"

	"github.com/smartystreets/gunit"

	"bitbucket.org/smartystreets/satisfy/fs"
)

func TestIntegrityFixture(t *testing.T) {
	gunit.Run(new(IntegrityFixture), t)
}

type IntegrityFixture struct {
	*gunit.Fixture

	checker    *FileListingIntegrityChecker
	fileSystem *fs.InMemoryFileSystem
}

func (this *IntegrityFixture) Setup() {
	this.fileSystem = fs.NewInMemoryFileSystem()
	this.checker = NewFileListingIntegrityChecker(this.fileSystem)
}

func (this *IntegrityFixture) TestFileListingIntegrityCheck() {

}
