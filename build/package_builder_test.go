package build

import (
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/fs"
)

func TestPackageBuilderFixture(t *testing.T) {
	gunit.Run(new(PackageBuilderFixture), t)
}

type PackageBuilderFixture struct {
	*gunit.Fixture
	builder    *PackageBuilder
	fileSystem contracts.FileSystem
	archive    *FakeArchiveWriter
	hasher     *FakeHasher
}

func (this *PackageBuilderFixture) Setup() {
	this.fileSystem = fs.NewInMemoryFileSystem()
	this.archive = NewFakeArchiveWriter()
	this.hasher = NewFakeHasher()
	this.builder = NewPackageBuilder(this.fileSystem, this.archive, this.hasher)
}

func (this *PackageBuilderFixture) TestContentsAreInventoried() {
	this.fileSystem.WriteFile("file0.txt", []byte("a"))
	this.fileSystem.WriteFile("file1.txt", []byte("b"))
	this.fileSystem.WriteFile("sub/file0.txt", []byte("c"))

	err := this.builder.Build()

	this.So(err, should.BeNil)
	this.So(this.builder.Contents(), should.Resemble, []contracts.ArchiveItem{
		{Path: "file0.txt", Size: 1, MD5Checksum: []byte("a [HASHED]")},
		{Path: "file1.txt", Size: 1, MD5Checksum: []byte("b [HASHED]")},
		{Path: "sub/file0.txt", Size: 1, MD5Checksum: []byte("c [HASHED]")},
	})
}

func (this *PackageBuilderFixture) TestContentsAreArchived() {
	this.fileSystem.WriteFile("file0.txt", []byte("a"))
	this.fileSystem.WriteFile("file1.txt", []byte("bb"))
	this.fileSystem.WriteFile("sub/file0.txt", []byte("ccc"))

	err := this.builder.Build()

	this.So(err, should.BeNil)
	this.So(this.archive.items, should.Resemble, []*ArchiveItem{
		{
			path:     "file0.txt",
			size:     1,
			contents: []byte("a"),
		},
		{
			path:     "file1.txt",
			size:     2,
			contents: []byte("bb"),
		},
		{
			path:     "sub/file0.txt",
			size:     3,
			contents: []byte("ccc"),
		},
	})
	this.So(this.archive.closed, should.BeTrue)
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
	path string
	size int64
	contents []byte
}

type FakeArchiveWriter struct {
	items []*ArchiveItem
	current *ArchiveItem
	closed bool
}

func NewFakeArchiveWriter() *FakeArchiveWriter { return &FakeArchiveWriter{} }
func (this *FakeArchiveWriter) WriteHeader(path string, size int64) {
	if this.closed {
		return
	}
	this.current = &ArchiveItem{
		path:     path,
		size:     size,
	}
	this.items = append(this.items, this.current)
}
func (this *FakeArchiveWriter) Write(p []byte) (int, error) {
	this.current.contents = append(this.current.contents, p...)
	return len(p), nil
}
func (this *FakeArchiveWriter) Close() error {
	this.closed = true
	return nil
}
