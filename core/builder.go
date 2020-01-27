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
	this.logger.Printf("Adding \"%s\" to archive.", file.Path())
	header, err := this.buildHeader(file)
	if err != nil {
		return err
	}
	this.archive.WriteHeader(header)
	err = this.archiveContents(file)
	if err != nil {
		return err
	}
	this.contents = append(this.contents, this.buildArchiveEntry(file))
	return err
}

func (this *PackageBuilder) archiveContents(file contracts.FileInfo) error {
	reader := this.storage.Open(file.Path())
	defer func() { _ = reader.Close() }()
	writer := io.MultiWriter(this.hasher, this.archive)
	if file.Symlink() != "" {
		writer = this.hasher
	}
	_, err := io.Copy(writer, reader)

	return err
}

func (this *PackageBuilder) buildHeader(file contracts.FileInfo) (header contracts.ArchiveHeader, err error) {
	header.Name = file.Path()
	header.Size = file.Size()
	header.ModTime = file.ModTime()

	if file.Symlink() == "" {
		return header, nil
	}

	if this.outOfBounds(file) {
		return header, this.symlinkOutOfBoundError(file)
	}
	header.LinkName, err = relativeLinkSourcePath(file)
	return header, err
}

func relativeLinkSourcePath(file contracts.FileInfo) (string, error) {
	return filepath.Rel(filepath.Dir(file.Path()), file.Symlink())
}

func (this *PackageBuilder) symlinkOutOfBoundError(file contracts.FileInfo) error {
	return fmt.Errorf(
		"the file \"%s\" is a symlink that refers to \"%s\" which is outside of the configured root directory: \"%s\"",
		file.Path(),
		file.Symlink(),
		this.storage.RootPath())
}

func (this *PackageBuilder) buildArchiveEntry(file contracts.FileInfo) contracts.ArchiveItem {
	defer this.hasher.Reset()

	return contracts.ArchiveItem{
		Path:        file.Path(),
		Size:        this.determineFileSize(file),
		MD5Checksum: this.hasher.Sum(nil),
	}
}

func (this *PackageBuilder) determineFileSize(file contracts.FileInfo) int64 {
	size := file.Size()
	if file.Symlink() == "" {
		return size
	}
	linkName, _ := relativeLinkSourcePath(file)
	return int64(len(linkName))
}

func (this *PackageBuilder) Contents() []contracts.ArchiveItem {
	return this.contents
}

func (this *PackageBuilder) outOfBounds(info contracts.FileInfo) bool {
	relative, err := filepath.Rel(this.storage.RootPath(), info.Symlink())
	return err != nil || strings.HasPrefix(relative, "..")
}
