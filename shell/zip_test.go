package shell

import (
	"bytes"
	"compress/flate"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/contracts"
)

func TestZipArchiveFixture(t *testing.T) {
	gunit.Run(new(ZipArchiveFixture), t)
}

type ZipArchiveFixture struct {
	*gunit.Fixture
	buffer *bytes.Buffer
}

func (this *ZipArchiveFixture) Setup() {
	this.buffer = bytes.NewBuffer(nil)
}

func (this *ZipArchiveFixture) TestWrittenArchiveCanBeReadBack() {
	this.writeArchive(flate.BestCompression)
	reader := NewZipArchiveReader(bytes.NewReader(this.buffer.Bytes())).(*ZipArchiveReader)
	defer func() { _ = reader.Close() }()

	for _, entry := range zipFixtureEntries {
		header, err := reader.Next()
		this.So(err, should.BeNil)
		if err != nil {
			return
		}
		this.So(header.Name, should.Equal, entry.name)
		this.So(header.Size, should.Equal, int64(len(entry.contents)))
		this.So(header.Typeflag, should.Equal, byte(0))
		this.So(header.Mode, should.Equal, int64(0644))
		this.So(header.ModTime.Unix(), should.Equal, zipFixtureModTime.Unix())

		contents, err := io.ReadAll(reader)
		this.So(err, should.BeNil)
		this.So(string(contents), should.Equal, string(entry.contents))
	}

	_, err := reader.Next()
	this.So(err, should.Equal, io.EOF)
}

func (this *ZipArchiveFixture) TestCloseRemovesTemporaryArchiveFile() {
	this.writeArchive(flate.DefaultCompression)
	reader := NewZipArchiveReader(bytes.NewReader(this.buffer.Bytes())).(*ZipArchiveReader)
	tempName := reader.file.Name()

	_, statErr := os.Stat(tempName)
	this.So(statErr, should.BeNil)

	this.So(reader.Close(), should.BeNil)

	_, statErr = os.Stat(tempName)
	this.So(os.IsNotExist(statErr), should.BeTrue)
}

func (this *ZipArchiveFixture) TestEmptyArchiveYieldsEOF() {
	writer := NewZipArchiveWriter(this.buffer, flate.DefaultCompression)
	this.So(writer.Close(), should.BeNil)

	reader := NewZipArchiveReader(bytes.NewReader(this.buffer.Bytes())).(*ZipArchiveReader)
	defer func() { _ = reader.Close() }()

	_, err := reader.Next()
	this.So(err, should.Equal, io.EOF)
}

func (this *ZipArchiveFixture) TestMalformedArchiveReportsErrorFromNext() {
	reader := NewZipArchiveReader(strings.NewReader("this is not a zip archive")).(*ZipArchiveReader)
	defer func() { _ = reader.Close() }()

	_, err := reader.Next()
	this.So(err, should.NotBeNil)
	this.So(err, should.NotEqual, io.EOF)
}

func (this *ZipArchiveFixture) TestConfiguredCompressionLevelIsApplied() {
	uncompressed := this.archiveSizeAtLevel(flate.NoCompression)
	compressed := this.archiveSizeAtLevel(flate.BestCompression)
	this.So(compressed, should.BeLessThan, uncompressed)
}

// UploadApp closes the compressor after PackageBuilder.Build has already
// closed the archive writer; for zip these are the same object.
func (this *ZipArchiveFixture) TestWriterCloseIsIdempotent() {
	writer := NewZipArchiveWriter(this.buffer, flate.DefaultCompression)
	writer.WriteHeader(contracts.ArchiveHeader{
		Name:    "file.txt",
		Size:    int64(len("contents")),
		ModTime: zipFixtureModTime,
	})
	_, _ = writer.Write([]byte("contents"))

	this.So(writer.Close(), should.BeNil)
	this.So(writer.Close(), should.BeNil)

	reader := NewZipArchiveReader(bytes.NewReader(this.buffer.Bytes())).(*ZipArchiveReader)
	defer func() { _ = reader.Close() }()

	header, err := reader.Next()
	this.So(err, should.BeNil)
	if err != nil {
		return
	}
	this.So(header.Name, should.Equal, "file.txt")
	contents, _ := io.ReadAll(reader)
	this.So(string(contents), should.Equal, "contents")
}

func (this *ZipArchiveFixture) TestReadAtServesTheDownloadedArchive() {
	this.writeArchive(flate.DefaultCompression)
	reader := NewZipArchiveReader(bytes.NewReader(this.buffer.Bytes())).(*ZipArchiveReader)
	defer func() { _ = reader.Close() }()

	signature := make([]byte, 4)
	n, err := reader.ReadAt(signature, 0)
	this.So(n, should.Equal, 4)
	this.So(err, should.BeNil)
	this.So(signature, should.Resemble, []byte("PK\x03\x04"))
}

func (this *ZipArchiveFixture) writeArchive(level int) {
	writer := NewZipArchiveWriter(this.buffer, level)
	for _, entry := range zipFixtureEntries {
		writer.WriteHeader(contracts.ArchiveHeader{
			Name:    entry.name,
			Size:    int64(len(entry.contents)),
			ModTime: zipFixtureModTime,
		})
		_, err := writer.Write(entry.contents)
		this.So(err, should.BeNil)
	}
	this.So(writer.Close(), should.BeNil)
}

func (this *ZipArchiveFixture) archiveSizeAtLevel(level int) int {
	buffer := bytes.NewBuffer(nil)
	writer := NewZipArchiveWriter(buffer, level)
	payload := bytes.Repeat([]byte("compressible content "), 1024)
	writer.WriteHeader(contracts.ArchiveHeader{
		Name:    "payload.txt",
		Size:    int64(len(payload)),
		ModTime: zipFixtureModTime,
	})
	_, _ = writer.Write(payload)
	_ = writer.Close()
	return buffer.Len()
}

var zipFixtureModTime = time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

var zipFixtureEntries = []struct {
	name     string
	contents []byte
}{
	{name: "greeting.txt", contents: []byte("Hello, Zip!")},
	{name: "empty.txt", contents: nil},
	{name: "nested/path/data.bin", contents: buildBinaryPayload()},
}

func buildBinaryPayload() []byte {
	payload := make([]byte, 64*1024)
	for i := range payload {
		payload[i] = byte(i * 7 % 253)
	}
	return payload
}
