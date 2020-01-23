package core

import (
	"hash"
	"io"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type PackageBuilderFileSystem interface {
	contracts.PathLister2
	contracts.FileOpener
}

type PackageBuilder struct {
	storage  PackageBuilderFileSystem
	archive  contracts.ArchiveWriter
	hasher   hash.Hash
	contents []contracts.ArchiveItem
}

func NewPackageBuilder(storage PackageBuilderFileSystem, archive contracts.ArchiveWriter, hasher hash.Hash) *PackageBuilder {
	return &PackageBuilder{
		storage: storage,
		archive: archive,
		hasher:  hasher,
	}
}

func (this *PackageBuilder) Build() error {
	listing, err := this.storage.Listing2()
	if err != nil {
		return err // TODO: wrap
	}
	for _, file := range listing {
		err := this.add(file)
		if err != nil {
			return err
		}
	}
	return this.archive.Close()
}

func (this *PackageBuilder) add(file contracts.FileInfo) error {
	this.archive.WriteHeader(contracts.ArchiveHeader{
		Name:    file.Path(),
		Size:    file.Size(),
		ModTime: file.ModTime(),
	})
	reader := this.storage.Open(file.Path())
	writer := io.MultiWriter(this.hasher, this.archive)
	_, err := io.Copy(writer, reader)
	this.contents = append(this.contents, this.buildArchiveEntry(file))
	return err
}

func (this *PackageBuilder) buildArchiveEntry(file contracts.FileInfo) contracts.ArchiveItem {
	defer this.hasher.Reset()
	return contracts.ArchiveItem{
		Path:        file.Path(),
		Size:        file.Size(),
		MD5Checksum: this.hasher.Sum(nil),
	}
}

func (this *PackageBuilder) Contents() []contracts.ArchiveItem {
	return this.contents
}
