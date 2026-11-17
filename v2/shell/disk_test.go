package shell

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestDiskFileSystemFixture(t *testing.T) {
	gunit.Run(new(DiskFileSystemFixture), t)
}

type DiskFileSystemFixture struct {
	*gunit.Fixture
	root string
	disk *DiskFileSystem
}

func (this *DiskFileSystemFixture) Setup() {
	root, err := os.MkdirTemp("", "satisfy-disk-test-*")
	this.So(err, should.BeNil)
	this.root = root
	this.disk = NewDiskFileSystem(this.root)
}

func (this *DiskFileSystemFixture) Teardown() {
	_ = os.RemoveAll(this.root)
}

func (this *DiskFileSystemFixture) path(name string) string {
	return filepath.Join(this.root, name)
}

func (this *DiskFileSystemFixture) TestWriteThenOpen() {
	this.So(this.disk.WriteFile(this.path("a/b.txt"), []byte("hello")), should.BeNil)
	reader, err := this.disk.Open(this.path("a/b.txt"))
	this.So(err, should.BeNil)
	defer func() { _ = reader.Close() }()
	raw, _ := io.ReadAll(reader)
	this.So(string(raw), should.Equal, "hello")
}

func (this *DiskFileSystemFixture) TestCreateMakesParentDirectories() {
	writer, err := this.disk.Create(this.path("x/y/z.txt"))
	this.So(err, should.BeNil)
	_, _ = writer.Write([]byte("z"))
	this.So(writer.Close(), should.BeNil)
	listing, err := this.disk.Listing()
	this.So(err, should.BeNil)
	this.So(listing, should.HaveLength, 1)
}

func (this *DiskFileSystemFixture) TestOpenMissingFileReturnsError() {
	reader, err := this.disk.Open(this.path("missing"))
	this.So(reader, should.BeNil)
	this.So(os.IsNotExist(err), should.BeTrue)
}

func (this *DiskFileSystemFixture) TestListingMissingRootReturnsError() {
	listing, err := NewDiskFileSystem(this.path("missing")).Listing()
	this.So(listing, should.BeNil)
	this.So(os.IsNotExist(err), should.BeTrue)
}

func (this *DiskFileSystemFixture) TestDeleteMissingFileReturnsError() {
	this.So(os.IsNotExist(this.disk.Delete(this.path("missing"))), should.BeTrue)
}

func (this *DiskFileSystemFixture) TestCreateSymlinkReplacesExistingTarget() {
	this.So(this.disk.WriteFile(this.path("target"), []byte("old")), should.BeNil)
	this.So(this.disk.CreateSymlink("source", this.path("target")), should.BeNil)
	info, err := this.disk.Stat(this.path("target"))
	this.So(err, should.BeNil)
	this.So(info.Symlink(), should.Equal, "source")
}
