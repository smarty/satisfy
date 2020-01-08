package fs

import (
	"io/ioutil"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
)

func TestMemoryFixture(t *testing.T) {
	gunit.Run(new(MemoryFixture), t)
}

type MemoryFixture struct {
	*gunit.Fixture
	fileSystem *InMemoryFileSystem
}

func (this *MemoryFixture) Setup() {
	this.fileSystem = NewInMemoryFileSystem()
}

func (this *MemoryFixture) TestWriteFileReadFile() {
	this.fileSystem.WriteFile("/file.txt", []byte("Hello World"))
	this.So(this.fileSystem.ReadFile("/file.txt"), should.Resemble, []byte("Hello World"))
}

func (this *MemoryFixture) TestReadFileNonExistingFile() {
	this.So(func() { this.fileSystem.ReadFile("/file.txt") }, should.Panic)
}

func (this *MemoryFixture) TestOpenWrittenFile() {
	this.fileSystem.WriteFile("/file.txt", []byte("Hello World"))
	reader := this.fileSystem.Open("/file.txt")
	raw, _ := ioutil.ReadAll(reader)
	this.So(raw, should.Resemble, []byte("Hello World"))
}

func (this *MemoryFixture) TestCreate() {
	writer := this.fileSystem.Create("/file.txt")
	_, _ = writer.Write([]byte("Hello World"))
	_ = writer.Close()
	this.So(this.fileSystem.ReadFile("/file.txt"), should.Resemble, []byte("Hello World"))
}

func (this *MemoryFixture) TestListing() {
	this.fileSystem.WriteFile("yes/file0.txt", []byte(""))
	this.fileSystem.WriteFile("yes/file1.txt", []byte("1"))
	this.fileSystem.WriteFile("no/file2.txt", []byte("12"))
	this.fileSystem.WriteFile("no/file3.txt", []byte("123"))

	fileInfo := this.fileSystem.Listing("yes")

	this.So(fileInfo, should.HaveLength, 2)
	this.So(fileInfo[0].Name(), should.Equal, "file0.txt")
	this.So(fileInfo[0].Size(), should.Equal, 0)
	this.So(fileInfo[1].Name(), should.Equal, "file1.txt")
	this.So(fileInfo[1].Size(), should.Equal, 1)
}
