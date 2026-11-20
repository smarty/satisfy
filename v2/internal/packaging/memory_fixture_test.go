package packaging

import (
	"bytes"
	"io"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestMemoryFixture(t *testing.T) {
	gunit.Run(new(MemoryFixture), t)
}

type MemoryFixture struct {
	*gunit.Fixture
	fileSystem *inMemoryFileSystem
}

func (this *MemoryFixture) Setup() {
	this.fileSystem = newInMemoryFileSystem()
}

func (this *MemoryFixture) TestWriteFileReadFile() {
	this.fileSystem.WriteFile("/file.txt", []byte("Hello World"))
	this.So(this.fileSystem.readFile("/file.txt"), should.Resemble, []byte("Hello World"))
}

func (this *MemoryFixture) TestSizeIsExhastingBuffer() {
	this.fileSystem.WriteFile("/file.txt", []byte("Hello World"))
	buffer := &bytes.Buffer{}
	reader, _ := this.fileSystem.Open("/file.txt")
	_, _ = io.Copy(buffer, reader)
	this.So(this.fileSystem.listing()[0].Size(), should.Equal, len([]byte("Hello World")))
}

func (this *MemoryFixture) TestReadFileNonExistingFile() {
	this.So(func() { this.fileSystem.readFile("/file.txt") }, should.Panic)
}

func (this *MemoryFixture) TestOpenWrittenFile() {
	this.fileSystem.WriteFile("/file.txt", []byte("Hello World"))
	reader, _ := this.fileSystem.Open("/file.txt")
	raw, _ := io.ReadAll(reader)
	this.So(raw, should.Resemble, []byte("Hello World"))
}

func (this *MemoryFixture) TestCreate() {
	writer, _ := this.fileSystem.Create("/file.txt")
	_, _ = writer.Write([]byte("Hello World"))
	_ = writer.Close()
	this.So(this.fileSystem.readFile("/file.txt"), should.Resemble, []byte("Hello World"))
}

func (this *MemoryFixture) TestListing() {
	this.fileSystem.WriteFile("file0.txt", []byte(""))
	this.fileSystem.WriteFile("file1.txt", []byte("1"))
	this.fileSystem.WriteFile("sub/file0.txt", []byte("12"))

	fileInfo := this.fileSystem.listing()

	this.So(fileInfo, should.HaveLength, 3)
	this.So(fileInfo[0].Path(), should.Equal, "file0.txt")
	this.So(fileInfo[0].Size(), should.Equal, 0)
	this.So(fileInfo[1].Path(), should.Equal, "file1.txt")
	this.So(fileInfo[1].Size(), should.Equal, 1)
	this.So(fileInfo[2].Path(), should.Equal, "sub/file0.txt")
	this.So(fileInfo[2].Size(), should.Equal, 2)
}

func (this *MemoryFixture) TestDelete() {
	this.fileSystem.WriteFile("/file.txt", []byte("Hello World"))

	this.fileSystem.Delete("/file.txt")

	this.So(this.fileSystem.listing(), should.BeEmpty)
}

func (this *MemoryFixture) TestCreateSymlink() {
	this.fileSystem.WriteFile("/source.txt", []byte("Hello World"))

	this.fileSystem.CreateSymlink("/source.txt", "/target.txt")

	this.So(this.fileSystem.listing(), should.HaveLength, 2)
	this.So(this.fileSystem.readFile("/target.txt"), should.Resemble, []byte("Hello World"))
}
