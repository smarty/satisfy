package core

import (
	"bytes"
	"crypto/md5"
	"io"
	"math/rand/v2"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/shell"
)

// ParallelZstdRoundTripFixture runs the same pieces as `satisfy upload`
// (DirectoryPackageBuilder → tar → ParallelZstdWriter, hashing the compressed
// stream) and then `satisfy download` (PackageInstaller's zstd reader and
// checksum verification), with a chunk size small enough that the archive is
// split into many independent zstd frames and files straddle frame boundaries.
func TestParallelZstdRoundTripFixture(t *testing.T) {
	gunit.Run(new(ParallelZstdRoundTripFixture), t)
}

type ParallelZstdRoundTripFixture struct {
	*gunit.Fixture
	source      *inMemoryFileSystem
	destination *inMemoryFileSystem
	downloader  *FakeDownloader
	files       map[string][]byte
}

const roundTripChunkSize = 4 << 10

func (this *ParallelZstdRoundTripFixture) Setup() {
	this.source = newInMemoryFileSystem()
	this.source.Root = "/in"
	this.destination = newInMemoryFileSystem()
	this.downloader = &FakeDownloader{}
	this.files = map[string][]byte{
		"big.txt":        roundTripContents(1, 50<<10),             // spans ~12 frames on its own
		"exact.txt":      roundTripContents(2, roundTripChunkSize), // one chunk's worth, misaligned by the tar header
		"small.txt":      []byte("Hello, World!"),
		"empty.txt":      nil,
		"nested/bin.dat": roundTripRandomBytes(3, 9<<10), // incompressible
	}
	for name, contents := range this.files {
		this.source.WriteFile("/in/"+name, contents)
	}
	_ = this.source.Chmod("/in/nested/bin.dat", 0755)
	this.source.CreateSymlink("/in/small.txt", "/in/link.txt")
}

func (this *ParallelZstdRoundTripFixture) TestMultiFrameArchiveInstallsIdenticalFiles() {
	for _, level := range []zstd.EncoderLevel{zstd.SpeedFastest, zstd.SpeedBestCompression} {
		this.destination = newInMemoryFileSystem()
		archive, manifest := this.upload(level, 4)

		this.So(this.frameContentSize(archive), should.Equal, roundTripChunkSize)
		this.So(this.install(archive, manifest), should.BeNil)
		this.assertInstalledFilesMatchSource()
	}
}

func (this *ParallelZstdRoundTripFixture) TestOutputDoesNotDependOnWorkerCount() {
	// Memory-constrained hosts run fewer workers; their archives must be
	// byte-for-byte the same so checksums agree across build machines.
	single, _ := this.upload(zstd.SpeedDefault, 1)
	many, _ := this.upload(zstd.SpeedDefault, 8)

	this.So(bytes.Equal(single, many), should.BeTrue)
}

func (this *ParallelZstdRoundTripFixture) TestCorruptedFrameFailsInstallAndRevertsFiles() {
	archive, manifest := this.upload(zstd.SpeedFastest, 4)
	archive[len(archive)/2] ^= 0xFF // somewhere in a middle frame

	err := this.install(archive, manifest)

	this.So(err, should.NotBeNil)
	this.So(this.destination.Listing(), should.BeEmpty)
}

// upload mirrors UploadApp.buildArchiveAndManifestContents.
func (this *ParallelZstdRoundTripFixture) upload(level zstd.EncoderLevel, workers int) ([]byte, contracts.Manifest) {
	compressed := new(bytes.Buffer)
	archiveHasher := md5.New()
	compressor, err := shell.NewParallelZstdWriterWithConcurrency(io.MultiWriter(archiveHasher, compressed), level, roundTripChunkSize, workers)
	this.So(err, should.BeNil)

	builder := NewDirectoryPackageBuilder(this.source, shell.NewSwitchArchiveWriter(compressor), md5.New(), false)
	this.So(builder.Build(), should.BeNil)
	this.So(compressor.Close(), should.BeNil)

	for _, item := range builder.Contents() {
		if contents, found := this.files[item.Path]; found {
			expected := md5.Sum(contents)
			this.So(item.MD5Checksum, should.Resemble, expected[:])
		}
	}
	return compressed.Bytes(), contracts.Manifest{Archive: contracts.Archive{
		MD5Checksum:          archiveHasher.Sum(nil),
		Contents:             builder.Contents(),
		CompressionAlgorithm: "zstd",
	}}
}

func (this *ParallelZstdRoundTripFixture) install(archive []byte, manifest contracts.Manifest) error {
	this.downloader.Body = io.NopCloser(bytes.NewReader(archive))
	installer := NewPackageInstaller(this.downloader, this.destination, false)
	return installer.InstallPackage(manifest, contracts.InstallationRequest{LocalPath: "out"})
}

func (this *ParallelZstdRoundTripFixture) assertInstalledFilesMatchSource() {
	for name, contents := range this.files {
		this.So(this.destination.readFile("out/"+name), should.Resemble, contents)
	}
	this.So(this.destination.fileSystem["out/nested/bin.dat"].mode, should.Equal, 0755)
	this.So(this.destination.fileSystem["out/link.txt"].symlink, should.Equal, "small.txt")
}

// frameContentSize reads the first frame header; a full chunk (rather than the
// whole archive) proves the stream was split into independent frames.
func (this *ParallelZstdRoundTripFixture) frameContentSize(archive []byte) uint64 {
	var header zstd.Header
	this.So(header.Decode(archive), should.BeNil)
	this.So(header.HasFCS, should.BeTrue)
	return header.FrameContentSize
}

func roundTripContents(seed uint64, size int) []byte {
	random := rand.New(rand.NewPCG(seed, seed))
	words := []string{"satisfy ", "upload ", "download ", "frame ", "chunk ", "manifest\n"}
	contents := make([]byte, 0, size+16)
	for len(contents) < size {
		contents = append(contents, words[random.IntN(len(words))]...)
	}
	return contents[:size]
}

func roundTripRandomBytes(seed uint64, size int) []byte {
	random := rand.New(rand.NewPCG(seed, seed))
	contents := make([]byte, size)
	for i := range contents {
		contents[i] = byte(random.Uint32())
	}
	return contents
}
