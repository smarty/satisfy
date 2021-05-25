package core

import (
	"hash"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
	"github.com/smartystreets/satisfy/contracts"
)

func TestFileContentIntegrityCheckFixture(t *testing.T) {
	gunit.Run(new(FileContentIntegrityCheckFixture), t)
}

type FileContentIntegrityCheckFixture struct {
	*gunit.Fixture

	checker    *FileContentIntegrityCheck
	fakeHasher *FakeHasher
	fileSystem *inMemoryFileSystem
	manifest   contracts.Manifest
}

func (this *FileContentIntegrityCheckFixture) Setup() {
	this.fakeHasher = NewFakeHasher()
	this.fileSystem = newInMemoryFileSystem()
	this.fileSystem.WriteFile("/local/a", []byte("a"))
	this.fileSystem.WriteFile("/local/bb", []byte("bb"))
	this.fileSystem.WriteFile("/local/cc/c", []byte("ccc"))
	this.fileSystem.WriteFile("/local/dddd", []byte("dddd"))
	this.fileSystem.CreateSymlink("cc/c", "/local/eeeee")

	this.manifest = contracts.Manifest{
		Archive: contracts.Archive{
			Contents: []contracts.ArchiveItem{
				{Path: "/a", MD5Checksum: []byte("a [HASHED]")},
				{Path: "/bb", MD5Checksum: []byte("bb [HASHED]")},
				{Path: "/cc/c", MD5Checksum: []byte("ccc [HASHED]")},
				{Path: "/dddd", MD5Checksum: []byte("dddd [HASHED]")},
				{Path: "/eeeee", MD5Checksum: []byte("cc/c [HASHED]")},
			},
		},
	}

	this.checker = NewFileContentIntegrityCheck(this.newHasher, this.fileSystem, false)
}

func (this *FileContentIntegrityCheckFixture) newHasher() hash.Hash {
	this.fakeHasher.Reset()
	return this.fakeHasher
}

func (this *FileContentIntegrityCheckFixture) TestFileContentsIntact() {
	this.checker.enabled = true

	this.So(this.checker.Verify(this.manifest, "/local"), should.BeNil)
}

func (this *FileContentIntegrityCheckFixture) TestIncorrectFileContentsCauseErrorWhenEnabled() {
	this.checker.enabled = true
	this.fileSystem.WriteFile("/local/bb", []byte("modified"))

	this.So(this.checker.Verify(this.manifest, "/local"), should.NotBeNil)
}

func (this *FileContentIntegrityCheckFixture) TestIncorrectFileContentsIgnoredWhenDisabled() {
	this.fileSystem.WriteFile("/local/bb", []byte("modified"))

	this.So(this.checker.Verify(this.manifest, "/local"), should.BeNil)
}
