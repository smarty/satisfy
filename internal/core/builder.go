package core

import (
	"fmt"
	"hash"
	"io"
	"log"
	"path/filepath"
	"strings"

	"github.com/smarty/satisfy/legacy_contracts"
)

type PackageBuilder interface {
	Build() error
	Contents() []legacy_contracts.ArchiveItem
}

type DirectoryPackageBuilderFileSystem interface {
	legacy_contracts.PathLister
	legacy_contracts.FileOpener
	legacy_contracts.RootPath
}

type DirectoryPackageBuilder struct {
	storage     DirectoryPackageBuilderFileSystem
	archive     legacy_contracts.ArchiveWriter
	hasher      hash.Hash
	contents    []legacy_contracts.ArchiveItem
	newProgress func(int64) io.WriteCloser
}

func NewDirectoryPackageBuilder(storage DirectoryPackageBuilderFileSystem, archive legacy_contracts.ArchiveWriter, hasher hash.Hash, newProgress func(int64) io.WriteCloser) PackageBuilder {
	if newProgress == nil {
		newProgress = noopProgress
	}

	return &DirectoryPackageBuilder{
		storage:     storage,
		archive:     archive,
		hasher:      hasher,
		newProgress: newProgress,
	}
}

func (this *DirectoryPackageBuilder) Build() error {
	if fileInfo, ok := this.fileOnly(); ok == true {
		err := this.add(fileInfo, true)
		if err != nil {
			return err
		}
	} else {
		for _, file := range this.storage.Listing() {
			err := this.add(file, false)
			if err != nil {
				return err
			}
		}
	}
	return this.archive.Close()
}

func (this *DirectoryPackageBuilder) add(file legacy_contracts.FileInfo, fileOnly bool) error {
	log.Printf("Adding \"%s\" to archive.", file.Path())
	header, err := this.buildHeader(file, fileOnly)
	if err != nil {
		return err
	}
	this.archive.WriteHeader(header)
	err = this.archiveContents(file, header.LinkName)
	if err != nil {
		return err
	}
	this.contents = append(this.contents, this.buildManifestEntry(file, header.LinkName))
	return err
}

func (this *DirectoryPackageBuilder) archiveContents(file legacy_contracts.FileInfo, symlinkSourcePath string) error {
	if symlinkSourcePath != "" {
		_, _ = io.WriteString(this.hasher, symlinkSourcePath)
		return nil
	}
	progressWriter := this.newProgress(file.Size())
	defer closeResource(progressWriter)
	writer := io.MultiWriter(this.hasher, this.archive, progressWriter)
	reader := this.storage.Open(file.Path())
	defer closeResource(reader)
	_, err := io.Copy(writer, reader)

	return err
}

func (this *DirectoryPackageBuilder) buildHeader(file legacy_contracts.FileInfo, fileOnly bool) (header legacy_contracts.ArchiveHeader, err error) {
	if fileOnly {
		header.Name = filepath.Base(file.Path())
	} else {
		header.Name = strings.TrimPrefix(file.Path(), this.storage.RootPath()+"/")
	}
	header.Size = file.Size()
	header.ModTime = file.ModTime()
	header.Executable = legacy_contracts.IsExecutable(file.Mode())
	if file.Symlink() == "" {
		return header, nil
	}

	if this.outOfBounds(file) {
		return header, this.symlinkOutOfBoundError(file)
	}
	header.LinkName, err = this.relativeLinkSourcePath(file)
	return header, err
}

func (this *DirectoryPackageBuilder) relativeLinkSourcePath(file legacy_contracts.FileInfo) (string, error) {
	path := file.Symlink()
	if this.isAbsolute(path) {
		return filepath.Rel(filepath.Dir(file.Path()), path)
	}
	joined := filepath.Join(filepath.Dir(file.Path()), path)
	path = filepath.Clean(joined)
	return filepath.Rel(filepath.Dir(file.Path()), path)
}

func (this *DirectoryPackageBuilder) symlinkOutOfBoundError(file legacy_contracts.FileInfo) error {
	return fmt.Errorf(
		"the file \"%s\" is a symlink that refers to \"%s\" which is outside of the configured root directory: \"%s\"",
		file.Path(),
		file.Symlink(),
		this.storage.RootPath())
}

func (this *DirectoryPackageBuilder) buildManifestEntry(file legacy_contracts.FileInfo, symlinkSourcePath string) legacy_contracts.ArchiveItem {
	defer this.hasher.Reset()
	var path string
	if _, ok := this.fileOnly(); ok == true {
		path = filepath.Base(file.Path())
	} else {
		path = strings.TrimPrefix(file.Path(), this.storage.RootPath()+"/")
	}

	return legacy_contracts.ArchiveItem{
		Path:        path,
		Size:        this.determineFileSize(file, symlinkSourcePath),
		MD5Checksum: this.hasher.Sum(nil),
	}
}

func (this *DirectoryPackageBuilder) determineFileSize(file legacy_contracts.FileInfo, symlinkSourcePath string) int64 {
	if symlinkSourcePath == "" {
		return file.Size()
	}
	return int64(len(symlinkSourcePath))
}

func (this *DirectoryPackageBuilder) Contents() []legacy_contracts.ArchiveItem {
	return this.contents
}

func (this *DirectoryPackageBuilder) outOfBounds(info legacy_contracts.FileInfo) bool {
	if this.isAbsolute(info.Symlink()) {
		return !strings.HasPrefix(info.Symlink(), this.storage.RootPath())
	}
	cleaned := filepath.Clean(filepath.Join(filepath.Dir(info.Path()), info.Symlink()))
	return !strings.HasPrefix(cleaned, this.storage.RootPath())
}

func (this *DirectoryPackageBuilder) isAbsolute(path string) bool {
	return strings.HasPrefix(path, "/")
}

func (this *DirectoryPackageBuilder) fileOnly() (legacy_contracts.FileInfo, bool) {
	if len(this.storage.Listing()) == 1 {
		if this.storage.Listing()[0].Mode().IsRegular() {
			return this.storage.Listing()[0], true
		}
	}
	return nil, false
}
