package build

import (
	"crypto/md5"
	"hash"
	"io"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type PackageBuilder struct {
	storage  contracts.FileSystem
	archive  contracts.ArchiveWriter
	contents []contracts.ArchiveItem
	hasher   hash.Hash
}

func NewPackageBuilder(storage contracts.FileSystem, archive contracts.ArchiveWriter) *PackageBuilder {
	return &PackageBuilder{
		storage: storage,
		archive: archive,
		hasher:  md5.New(),
	}
}

func (this *PackageBuilder) Build() error {
	for _, file := range this.storage.Listing() {
		this.hasher.Reset()
		reader := this.storage.Open(file.Path())
		_, _ = io.Copy(this.hasher, reader)
		this.contents = append(this.contents, contracts.ArchiveItem{
			Path:        file.Path(),
			Size:        file.Size(),
			MD5Checksum: this.hasher.Sum(nil),
		})
	}
	return nil
}

func (this *PackageBuilder) Contents() []contracts.ArchiveItem {
	return this.contents
}
