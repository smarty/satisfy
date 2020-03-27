package core

import (
	"errors"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
	"github.com/smartystreets/logging"
	"github.com/smartystreets/satisfy/contracts"
)

func TestPackageBuilderFixture(t *testing.T) {
	gunit.Run(new(PackageBuilderFixture), t)
}

type PackageBuilderFixture struct {
	*gunit.Fixture
	builder    *PackageBuilder
	fileSystem *inMemoryFileSystem
	archive    *FakeArchiveWriter
	hasher     *FakeHasher
}

func (this *PackageBuilderFixture) Setup() {
	this.fileSystem = newInMemoryFileSystem()
	this.archive = NewFakeArchiveWriter()
	this.hasher = NewFakeHasher()
	this.builder = NewPackageBuilder(this.fileSystem, this.archive, this.hasher)
	this.builder.logger = logging.Capture()
	this.fileSystem.WriteFile("/in/file0.txt", []byte("a"))
	_ = this.fileSystem.Chmod("/in/file0.txt", 0755)
	this.fileSystem.WriteFile("/in/file1.txt", []byte("bb"))
	this.fileSystem.CreateSymlink("/in/file0.txt", "/in/inner/link.txt")
	this.fileSystem.WriteFile("/in/sub/file0.txt", []byte("ccc"))
	this.fileSystem.Root = "/in"
}

func (this *PackageBuilderFixture) TestContentsAreInventoried() {
	err := this.builder.Build()

	this.So(err, should.BeNil)
	this.So(this.builder.Contents(), should.Resemble, []contracts.ArchiveItem{
		{Path: "file0.txt", Size: 1, MD5Checksum: []byte("a [HASHED]")},
		{Path: "file1.txt", Size: 2, MD5Checksum: []byte("bb [HASHED]")},
		{Path: "inner/link.txt", Size: 12, MD5Checksum: []byte("../file0.txt [HASHED]")},
		{Path: "sub/file0.txt", Size: 3, MD5Checksum: []byte("ccc [HASHED]")},
	})
}

func (this *PackageBuilderFixture) TestContentsAreArchived() {
	err := this.builder.Build()

	this.So(err, should.BeNil)
	this.So(this.archive.items, should.Resemble, []*ArchiveItem{
		{ArchiveHeader: contracts.ArchiveHeader{Name: "file0.txt", Size: 1, ModTime: InMemoryModTime, Executable: true}, contents: []byte("a")},
		{ArchiveHeader: contracts.ArchiveHeader{Name: "file1.txt", Size: 2, ModTime: InMemoryModTime}, contents: []byte("bb")},
		{ArchiveHeader: contracts.ArchiveHeader{Name: "inner/link.txt", LinkName: "../file0.txt", Size: 0, ModTime: InMemoryModTime}, contents: nil},
		{ArchiveHeader: contracts.ArchiveHeader{Name: "sub/file0.txt", Size: 3, ModTime: InMemoryModTime}, contents: []byte("ccc")},
	})
	this.So(this.archive.closed, should.BeTrue)
}

func (this *PackageBuilderFixture) TestSimulatedArchiveWriteError() {
	this.archive.writeError = writeErr

	err := this.builder.Build()

	this.So(err, should.Equal, writeErr)
}

func (this *PackageBuilderFixture) TestSimulatedArchiveCloseError() {
	this.archive.closedError = closeErr

	err := this.builder.Build()

	this.So(err, should.Equal, closeErr)
}

func (this *PackageBuilderFixture) TestAbsoluteSymlinkOutOfBoundsNotAllowed() {
	this.fileSystem.CreateSymlink("/out/of-bounds.txt", "/in/link.txt")
	err := this.builder.Build()
	this.So(err, should.NotBeNil)
}

func (this *PackageBuilderFixture) TestRelativeSymlinkOutOfBoundsNotAllowed() {
	this.fileSystem.CreateSymlink("../../out/of-bounds.txt", "/in/link.txt")
	err := this.builder.Build()
	this.So(err, should.NotBeNil)
}

func (this *PackageBuilderFixture) TestRelativeSymlinkInBoundsIsAllowed() {
	this.fileSystem.CreateSymlink("../file0.txt", "/in/inner/link.txt")
	err := this.builder.Build()
	if !this.So(err, should.BeNil) {
		return
	}
	this.So(this.archive.items[2], should.Resemble, &ArchiveItem{ArchiveHeader: contracts.ArchiveHeader{
		Name:     "inner/link.txt",
		LinkName: "../file0.txt",
		Size:     0,
		ModTime:  InMemoryModTime,
	}, contents: nil})
}

/////////////////////////

type FakeHasher struct{ sum []byte }

func NewFakeHasher() *FakeHasher { return &FakeHasher{} }
func (this *FakeHasher) Write(p []byte) (n int, err error) {
	this.sum = append(this.sum, p...)
	this.sum = append(this.sum, []byte(" [HASHED]")...)
	return len(p), nil
}
func (this *FakeHasher) Reset()              { this.sum = nil }
func (this *FakeHasher) Sum(b []byte) []byte { return this.sum }
func (this *FakeHasher) BlockSize() int      { panic("implement me") }
func (this *FakeHasher) Size() int           { panic("implement me") }

/////////////////////////

type ArchiveItem struct {
	contracts.ArchiveHeader
	contents []byte
}

type FakeArchiveWriter struct {
	items       []*ArchiveItem
	current     *ArchiveItem
	closed      bool
	writeError  error
	closedError error
}

func NewFakeArchiveWriter() *FakeArchiveWriter { return &FakeArchiveWriter{} }
func (this *FakeArchiveWriter) WriteHeader(header contracts.ArchiveHeader) {
	if this.closed {
		return
	}
	this.current = &ArchiveItem{ArchiveHeader: header}
	this.items = append(this.items, this.current)
}
func (this *FakeArchiveWriter) Write(p []byte) (int, error) {
	this.current.contents = append(this.current.contents, p...)
	return len(p), this.writeError

}
func (this *FakeArchiveWriter) Close() error {
	this.closed = true
	return this.closedError
}

var (
	writeErr = errors.New("write error")
	closeErr = errors.New("close error")
)
