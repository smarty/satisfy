package packaging

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2/internal/model"
)

func TestDirectoryPackageBuilderFixture(t *testing.T) {
	gunit.Run(new(DirectoryPackageBuilderFixture), t)
}

type DirectoryPackageBuilderFixture struct {
	*gunit.Fixture
	builder    PackageBuilder
	fileSystem *inMemoryFileSystem
	archive    *FakeArchiveWriter
	hasher     *FakeHasher
}

func (this *DirectoryPackageBuilderFixture) Setup() {
	this.fileSystem = newInMemoryFileSystem()
	this.archive = NewFakeArchiveWriter()
	this.hasher = NewFakeHasher()
	this.builder = NewDirectoryPackageBuilder(this.fileSystem, this.archive, this.hasher, nil, nil)
	this.fileSystem.WriteFile("/in/file0.txt", []byte("a"))
	_ = this.fileSystem.Chmod("/in/file0.txt", 0755)
	this.fileSystem.WriteFile("/in/file1.txt", []byte("bb"))
	this.fileSystem.CreateSymlink("/in/file0.txt", "/in/inner/link.txt")
	this.fileSystem.WriteFile("/in/sub/file0.txt", []byte("ccc"))
	this.fileSystem.Root = "/in"
}
func (this *DirectoryPackageBuilderFixture) TestContentsAreInventoried() {
	err := this.builder.Build(context.Background())

	this.So(err, should.BeNil)
	this.So(this.builder.Contents(), should.Resemble, []model.ArchiveItem{
		{Path: "file0.txt", Size: 1, MD5Checksum: []byte("a [HASHED]")},
		{Path: "file1.txt", Size: 2, MD5Checksum: []byte("bb [HASHED]")},
		{Path: "inner/link.txt", Size: 12, MD5Checksum: []byte("../file0.txt [HASHED]")},
		{Path: "sub/file0.txt", Size: 3, MD5Checksum: []byte("ccc [HASHED]")},
	})
}
func (this *DirectoryPackageBuilderFixture) TestExcludedFilesAreLeftOut() {
	var seen []string
	builder := NewDirectoryPackageBuilder(this.fileSystem, this.archive, this.hasher, nil, nil).
		WithExclusion(func(relativePath string) bool {
			seen = append(seen, relativePath)
			return relativePath == "file1.txt" || relativePath == "sub/file0.txt"
		})

	err := builder.Build(context.Background())

	this.So(err, should.BeNil)
	this.So(seen, should.Resemble, []string{"file0.txt", "file1.txt", "inner/link.txt", "sub/file0.txt"})
	this.So(builder.Contents(), should.Resemble, []model.ArchiveItem{
		{Path: "file0.txt", Size: 1, MD5Checksum: []byte("a [HASHED]")},
		{Path: "inner/link.txt", Size: 12, MD5Checksum: []byte("../file0.txt [HASHED]")},
	})
}
func (this *DirectoryPackageBuilderFixture) TestCancelledBuildStops() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := this.builder.Build(ctx)

	this.So(errors.Is(err, context.Canceled), should.BeTrue)
	this.So(this.builder.Contents(), should.BeEmpty)
}

func (this *DirectoryPackageBuilderFixture) TestCopyStopsWhenCancelled() {
	ctx, cancel := context.WithCancel(context.Background())
	reader := contextReader{ctx: ctx, reader: bytes.NewReader([]byte("content"))}
	buffer := make([]byte, 2)

	_, err := reader.Read(buffer)
	this.So(err, should.BeNil)
	cancel()
	_, err = reader.Read(buffer)
	this.So(err, should.Equal, context.Canceled)
}

func (this *DirectoryPackageBuilderFixture) TestListingErrorIsReturned() {
	listingErr := errors.New("listing failed")
	this.fileSystem.errListing = listingErr

	this.So(this.builder.Build(context.Background()), should.Equal, listingErr)
}
func (this *DirectoryPackageBuilderFixture) TestOpenErrorIsReturned() {
	openErr := errors.New("open failed")
	this.fileSystem.errOpen["/in/file1.txt"] = openErr

	this.So(this.builder.Build(context.Background()), should.Equal, openErr)
}
func (this *DirectoryPackageBuilderFixture) TestProgressIsReportedForEachArchivedFile() {
	var finished []string
	this.builder = NewDirectoryPackageBuilder(this.fileSystem, this.archive, this.hasher, nil, func(progress model.Progress) {
		if progress.Done {
			this.So(progress.Phase, should.Equal, model.ProgressArchiving)
			this.So(progress.Written, should.Equal, progress.Total)
			finished = append(finished, progress.Name)
		}
	})

	this.So(this.builder.Build(context.Background()), should.BeNil)

	this.So(finished, should.Resemble, []string{"/in/file0.txt", "/in/file1.txt", "/in/sub/file0.txt"})
}
func (this *DirectoryPackageBuilderFixture) TestContentsAreArchived() {
	err := this.builder.Build(context.Background())

	this.So(err, should.BeNil)
	this.So(this.archive.items, should.Resemble, []*ArchiveItem{
		{Name: "file0.txt", Size: 1, ModTime: InMemoryModTime, Executable: true, contents: []byte("a")},
		{Name: "file1.txt", Size: 2, ModTime: InMemoryModTime, contents: []byte("bb")},
		{Name: "inner/link.txt", LinkName: "../file0.txt", Size: 0, ModTime: InMemoryModTime, contents: nil},
		{Name: "sub/file0.txt", Size: 3, ModTime: InMemoryModTime, contents: []byte("ccc")},
	})
	this.So(this.archive.closed, should.BeTrue)
}
func (this *DirectoryPackageBuilderFixture) TestSimulatedArchiveWriteError() {
	this.archive.writeError = writeErr

	err := this.builder.Build(context.Background())

	this.So(err, should.Equal, writeErr)
}
func (this *DirectoryPackageBuilderFixture) TestSimulatedArchiveCloseError() {
	this.archive.closedError = closeErr

	err := this.builder.Build(context.Background())

	this.So(err, should.Equal, closeErr)
}
func (this *DirectoryPackageBuilderFixture) TestAbsoluteSymlinkOutOfBoundsNotAllowed() {
	this.fileSystem.CreateSymlink("/out/of-bounds.txt", "/in/link.txt")
	err := this.builder.Build(context.Background())
	this.So(err, should.NotBeNil)
}
func (this *DirectoryPackageBuilderFixture) TestRelativeSymlinkOutOfBoundsNotAllowed() {
	this.fileSystem.CreateSymlink("../../out/of-bounds.txt", "/in/link.txt")
	err := this.builder.Build(context.Background())
	this.So(err, should.NotBeNil)
}
func (this *DirectoryPackageBuilderFixture) TestRelativeSymlinkInBoundsIsAllowed() {
	this.fileSystem.CreateSymlink("../file0.txt", "/in/inner/link.txt")
	err := this.builder.Build(context.Background())
	if !this.So(err, should.BeNil) {
		return
	}
	this.So(this.archive.items[2], should.Resemble, &ArchiveItem{
		Name:     "inner/link.txt",
		LinkName: "../file0.txt",
		Size:     0,
		ModTime:  InMemoryModTime,
		contents: nil,
	})
}
func (this *DirectoryPackageBuilderFixture) TestFileOnlyEnsureNoPath() {
	this.fileSystem = newInMemoryFileSystem()
	this.archive = NewFakeArchiveWriter()
	this.hasher = NewFakeHasher()
	this.builder = NewDirectoryPackageBuilder(this.fileSystem, this.archive, this.hasher, nil, nil)
	this.fileSystem.WriteFile("/out/in/file0.txt", []byte("a"))
	err := this.builder.Build(context.Background())
	if !this.So(err, should.BeNil) {
		return
	}
	this.So(this.archive.items[0].ArchiveHeader.Name, should.Equal, "file0.txt")
}

func (this *DirectoryPackageBuilderFixture) TestMultipleFilesEnsurePath() {
	this.fileSystem = newInMemoryFileSystem()
	this.archive = NewFakeArchiveWriter()
	this.hasher = NewFakeHasher()
	this.builder = NewDirectoryPackageBuilder(this.fileSystem, this.archive, this.hasher, nil, nil)
	this.fileSystem.WriteDirectory("/out")
	this.fileSystem.WriteFile("/out/in/file0.txt", []byte("a"))
	this.fileSystem.WriteFile("/out/in/file1.txt", []byte("a"))
	err := this.builder.Build(context.Background())
	if !this.So(err, should.BeNil) {
		return
	}
	this.So(len(this.archive.items), should.Equal, 3)
	this.So(this.archive.items[0].ArchiveHeader.Name, should.Equal, "out")
	this.So(this.archive.items[1].ArchiveHeader.Name, should.Equal, "out/in/file0.txt")
	this.So(this.archive.items[2].ArchiveHeader.Name, should.Equal, "out/in/file1.txt")
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
	model.ArchiveHeader
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
func (this *FakeArchiveWriter) WriteHeader(header model.ArchiveHeader) error {
	if this.closed {
		return nil
	}
	this.current = &ArchiveItem{ArchiveHeader: header}
	this.items = append(this.items, this.current)
	return nil
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
