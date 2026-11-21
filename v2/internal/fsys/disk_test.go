package fsys

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2/internal/model"
)

func TestDiskFileSystemFixture(t *testing.T) {
	gunit.Run(new(DiskFileSystemFixture), t)
}

type DiskFileSystemFixture struct {
	*gunit.Fixture
	root  string
	disk  *DiskFileSystem
	roots []model.RootedFileSystem
}

func (this *DiskFileSystemFixture) Setup() {
	root, err := os.MkdirTemp("", "satisfy-disk-test-*")
	this.So(err, should.BeNil)
	this.root = root
	this.disk = NewDiskFileSystem(this.root)
}

func (this *DiskFileSystemFixture) Teardown() {
	for _, root := range this.roots {
		_ = root.Close()
	}
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

func (this *DiskFileSystemFixture) openRoot() model.RootedFileSystem {
	root, err := this.disk.OpenRoot(this.root)
	this.So(err, should.BeNil)
	this.roots = append(this.roots, root)
	return root
}

func (this *DiskFileSystemFixture) TestCreateMakesParentDirectories() {
	writer, err := this.openRoot().Create("x/y/z.txt")
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
	this.So(errors.Is(this.openRoot().Delete("missing"), fs.ErrNotExist), should.BeTrue)
}

func (this *DiskFileSystemFixture) TestCreateSymlinkReplacesExistingTarget() {
	this.So(this.disk.WriteFile(this.path("target"), []byte("old")), should.BeNil)
	this.So(this.openRoot().CreateSymlink("source", "target"), should.BeNil)
	info, err := this.disk.Stat(this.path("target"))
	this.So(err, should.BeNil)
	this.So(info.Symlink(), should.Equal, "source")
}

// Archives list entries in walk order, so a symlink can be extracted before any
// regular file in its directory has created that directory.
func (this *DiskFileSystemFixture) TestCreateSymlinkMakesParentDirectories() {
	this.So(this.openRoot().CreateSymlink("b.txt", "inner/a-link"), should.BeNil)
	info, err := this.disk.Stat(this.path("inner/a-link"))
	this.So(err, should.BeNil)
	this.So(info.Symlink(), should.Equal, "b.txt")
}

func (this *DiskFileSystemFixture) TestOpenRootConfinesWritesToTheDirectory() {
	root, err := this.disk.OpenRoot(this.path("install"))
	this.So(err, should.BeNil)
	defer func() { _ = root.Close() }()

	_, err = root.Create("../outside")
	this.So(err, should.NotBeNil)

	// Each link looks harmless on its own, but together they lead out of the
	// directory: "a/up" is the directory itself, so "a/up/b" -> ".." is its parent.
	this.So(root.CreateSymlink("..", "a/up"), should.BeNil)
	this.So(root.CreateSymlink("..", "a/up/b"), should.BeNil)
	_, err = root.Create("a/up/b/escaped")
	this.So(err, should.NotBeNil)
	_, statErr := os.Lstat(this.path("escaped"))
	this.So(os.IsNotExist(statErr), should.BeTrue)
}

func (this *DiskFileSystemFixture) TestOpenRootCreatesFilesSymlinksAndDeletes() {
	root, err := this.disk.OpenRoot(this.path("install"))
	this.So(err, should.BeNil)
	defer func() { _ = root.Close() }()

	writer, err := root.Create("dir/file")
	this.So(err, should.BeNil)
	_, _ = writer.Write([]byte("content"))
	this.So(writer.Close(), should.BeNil)
	this.So(root.Chmod("dir/file", 0755), should.BeNil)
	this.So(root.CreateSymlink("file", "dir/link"), should.BeNil)

	raw, err := os.ReadFile(this.path("install/dir/link"))
	this.So(err, should.BeNil)
	this.So(string(raw), should.Equal, "content")
	info, _ := os.Stat(this.path("install/dir/file"))
	this.So(info.Mode().Perm(), should.Equal, os.FileMode(0755))

	this.So(root.Delete("dir/link"), should.BeNil)
	this.So(root.Delete("dir/file"), should.BeNil)
	_, err = os.Lstat(this.path("install/dir/file"))
	this.So(os.IsNotExist(err), should.BeTrue)
}

func (this *DiskFileSystemFixture) TestStatOfRegularFileHasNoSymlink() {
	this.So(this.disk.WriteFile(this.path("plain"), []byte("x")), should.BeNil)
	info, err := this.disk.Stat(this.path("plain"))
	this.So(err, should.BeNil)
	this.So(info.Symlink(), should.BeEmpty)
	this.So(info.Size(), should.Equal, int64(1))
}
