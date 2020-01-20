package core

import (
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/shell"
)

func TestFileContentIntegrityCheckFixture(t *testing.T) {
	gunit.Run(new(FileContentIntegrityCheckFixture), t)
}

type FileContentIntegrityCheckFixture struct {
	*gunit.Fixture

	checker    *FileContentIntegrityCheck
	fakeHasher *FakeHasher
	fileSystem *shell.InMemoryFileSystem
	manifest   contracts.Manifest
}

func (this *FileContentIntegrityCheckFixture) Setup() {
	this.fakeHasher = NewFakeHasher()
	this.fileSystem = shell.NewInMemoryFileSystem()
	this.fileSystem.WriteFile("/a", []byte("a"))
	this.fileSystem.WriteFile("/bb", []byte("bb"))
	this.fileSystem.WriteFile("/cc/c", []byte("ccc"))
	this.fileSystem.WriteFile("/dddd", []byte("dddd"))

	this.manifest = contracts.Manifest{
		Archive: contracts.Archive{
			Contents: []contracts.ArchiveItem{
				{Path: "/a", MD5Checksum: []byte("a [HASHED]")},
				{Path: "/bb", MD5Checksum: []byte("bb [HASHED]")},
				{Path: "/cc/c", MD5Checksum: []byte("ccc [HASHED]")},
				{Path: "/dddd", MD5Checksum: []byte("dddd [HASHED]")},
			},
		},
	}

	this.checker = NewFileContentIntegrityCheck(this.fakeHasher, this.fileSystem, false)
}

func (this *FileContentIntegrityCheckFixture) TestFileContentsIntact() {
	this.So(this.checker.Verify(this.manifest), should.BeNil)
}

func (this *FileContentIntegrityCheckFixture) TestIncorrectFileContentsCauseErrorWhenEnabled() {
	this.checker.enabled = true
	this.fileSystem.WriteFile("/bb", []byte("modified"))

	this.So(this.checker.Verify(this.manifest), should.NotBeNil)
}

func (this *FileContentIntegrityCheckFixture) TestIncorrectFileContentsIgnoredWhenDisabled() {
	this.fileSystem.WriteFile("/bb", []byte("modified"))

	this.So(this.checker.Verify(this.manifest), should.BeNil)
}
