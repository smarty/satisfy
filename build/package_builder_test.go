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
}

func (this *PackageBuilderFixture) Setup() {
	this.fileSystem = fs.NewInMemoryFileSystem()
	this.archive = NewFakeArchiveWriter()
	this.builder = NewPackageBuilder(this.fileSystem, this.archive)
}

func (this *PackageBuilderFixture) TestArchiveContentsAreInventoried() {
	this.fileSystem.WriteFile("file0.txt", []byte(""))
	this.fileSystem.WriteFile("file1.txt", []byte("1"))
	this.fileSystem.WriteFile("sub/file0.txt", []byte("12"))

	err := this.builder.Build()
	this.So(err, should.BeNil)
	this.So(this.builder.Contents(), should.Resemble, []contracts.ArchiveItem{
		{Path: "file0.txt", Size: 0, MD5Checksum: []uint8{212, 29, 140, 217, 143, 0, 178, 4, 233, 128, 9, 152, 236, 248, 66, 126}},
		{Path: "file1.txt", Size: 1, MD5Checksum: []uint8{196, 202, 66, 56, 160, 185, 35, 130, 13, 204, 80, 154, 111, 117, 132, 155}},
		{Path: "sub/file0.txt", Size: 2, MD5Checksum: []uint8{194, 10, 212, 215, 111, 233, 119, 89, 170, 39, 160, 201, 155, 255, 103, 16}},
	})
}

/////////////////////////

type FakeArchiveWriter struct {
}

func NewFakeArchiveWriter() *FakeArchiveWriter {
	return &FakeArchiveWriter{}
}

func (this *FakeArchiveWriter) Write([]byte) (int, error) {
	panic("implement me")
}

func (this *FakeArchiveWriter) Close() error {
	panic("implement me")
}

func (this *FakeArchiveWriter) WriteHeader(name string, size int64) {
	panic("implement me")
}
