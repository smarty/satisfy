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

