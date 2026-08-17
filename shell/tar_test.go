package shell

import (
	"archive/tar"
	"bytes"
	"compress/flate"
	"io"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/contracts"
)

func TestTarArchiveWriterFixture(t *testing.T) {
	gunit.Run(new(TarArchiveWriterFixture), t)
}

type TarArchiveWriterFixture struct {
	*gunit.Fixture
	buffer *bytes.Buffer
}

func (this *TarArchiveWriterFixture) Setup() {
	this.buffer = bytes.NewBuffer(nil)
}

func (this *TarArchiveWriterFixture) TestSwitchReturnsArchiveWriterUnchanged() {
	zipWriter := NewZipArchiveWriter(this.buffer, flate.DefaultCompression)
	this.So(NewSwitchArchiveWriter(zipWriter), should.Equal, zipWriter)
}

func (this *TarArchiveWriterFixture) TestSwitchWrapsPlainWriterInTar() {
	this.So(NewSwitchArchiveWriter(this.buffer), should.HaveSameTypeAs, &TarArchiveWriter{})
}

func (this *TarArchiveWriterFixture) TestWrittenArchiveCanBeReadBack() {
	writer := NewTarArchiveWriter(this.buffer)
	writer.WriteHeader(contracts.ArchiveHeader{
		Name: "Hello/World",
		Size: int64(len("Hello World")),
	})
	_, _ = writer.Write([]byte("Hello World"))
	writer.WriteHeader(contracts.ArchiveHeader{
		Name:       "bin/tool",
		Size:       int64(len("#!binary")),
		Executable: true,
	})
	_, _ = writer.Write([]byte("#!binary"))
	writer.WriteHeader(contracts.ArchiveHeader{
		Name:     "Link",
		LinkName: "Hello/World",
	})
	this.So(writer.Close(), should.BeNil)

	reader := tar.NewReader(this.buffer)

	header, err := reader.Next()
	this.So(err, should.BeNil)
	this.So(header.Name, should.Equal, "Hello/World")
	this.So(header.Mode, should.Equal, int64(0644))
	contents, _ := io.ReadAll(reader)
	this.So(string(contents), should.Equal, "Hello World")

	header, err = reader.Next()
	this.So(err, should.BeNil)
	this.So(header.Name, should.Equal, "bin/tool")
	this.So(header.Mode, should.Equal, int64(0755))
	contents, _ = io.ReadAll(reader)
	this.So(string(contents), should.Equal, "#!binary")

	header, err = reader.Next()
	this.So(err, should.BeNil)
	this.So(header.Name, should.Equal, "Link")
	this.So(header.Typeflag, should.Equal, byte(tar.TypeSymlink))
	this.So(header.Linkname, should.Equal, "Hello/World")

	_, err = reader.Next()
	this.So(err, should.Equal, io.EOF)
}
