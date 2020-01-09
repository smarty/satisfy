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
	archive    contracts.ArchiveWriter
	hasher     *FakeHasher
}

func (this *PackageBuilderFixture) Setup() {
	this.fileSystem = fs.NewInMemoryFileSystem()
	this.archive = NewFakeArchiveWriter()
	this.hasher = NewFakeHasher()
	this.builder = NewPackageBuilder(this.fileSystem, this.archive, this.hasher)
}

func (this *PackageBuilderFixture) TestArchiveContentsAreInventoried() {
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

type FakeArchiveWriter struct{}

func NewFakeArchiveWriter() *FakeArchiveWriter                      { return &FakeArchiveWriter{} }
func (this *FakeArchiveWriter) Write([]byte) (int, error)           { panic("implement me") }
func (this *FakeArchiveWriter) Close() error                        { panic("implement me") }
func (this *FakeArchiveWriter) WriteHeader(name string, size int64) { panic("implement me") }
