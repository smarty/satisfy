package core

import (
	"fmt"
	"hash"
	"io"
	"path/filepath"
	"strings"

	"github.com/smartystreets/logging"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type PackageBuilderFileSystem interface {
	contracts.PathLister
	contracts.FileOpener
	contracts.RootPath
}

type PackageBuilder struct {
	logger   *logging.Logger
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
	for _, file := range this.storage.Listing() {
		err := this.add(file)
		if err != nil {
			return err
		}
	}
	return this.archive.Close()
}

func (this *PackageBuilder) add(file contracts.FileInfo) error {
	if file.Symlink() != "" && this.outOfBounds(file) {
		return fmt.Errorf(
			"the file \"%s\" is a symlink that refers to \"%s\" which is outside of the configured root directory: \"%s\"",
			file.Path(),
			file.Symlink(),
			this.storage.RootPath())
	}
	this.logger.Printf("Adding \"%s\" to archive.", file.Path())
	this.archive.WriteHeader(contracts.ArchiveHeader{
		Name:     file.Path(),
		Size:     file.Size(),
		ModTime:  file.ModTime(),
		LinkName: file.Symlink(),
	})
	reader := this.storage.Open(file.Path())
	defer func() { _ = reader.Close() }()
	writer := io.MultiWriter(this.hasher, this.archive)
	if file.Symlink() != "" {
		writer = this.hasher
	}
	_, err := io.Copy(writer, reader)
	if err != nil {
		return err
	}
	this.contents = append(this.contents, this.buildArchiveEntry(file))
	return err
}

func (this *PackageBuilder) buildArchiveEntry(file contracts.FileInfo) contracts.ArchiveItem {
	defer this.hasher.Reset()

	size := file.Size()
	if file.Symlink() != "" {
		size = 1
	}
	return contracts.ArchiveItem{
		Path:        file.Path(),
		Size:        size,
		MD5Checksum: this.hasher.Sum(nil),
	}
}

func (this *PackageBuilder) Contents() []contracts.ArchiveItem {
	return this.contents
}

func (this *PackageBuilder) outOfBounds(info contracts.FileInfo) bool {
	relative, err := filepath.Rel(this.storage.RootPath(), info.Symlink())
	return err != nil || strings.HasPrefix(relative, "..")
}
