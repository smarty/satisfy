package core

import (
	"fmt"
	"hash"
	"io"
	"path/filepath"
	"strings"

	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/internal/plumbing"
)

type PackageBuilder interface {
	Build() error
	Contents() []plumbing.ArchiveItem
}

type DirectoryPackageBuilderFileSystem interface {
	plumbing.PathLister
	plumbing.FileOpener
	plumbing.RootPath
}

type DirectoryPackageBuilder struct {
	storage     DirectoryPackageBuilderFileSystem
	archive     plumbing.ArchiveWriter
	hasher      hash.Hash
	contents    []plumbing.ArchiveItem
	emit        func(contracts.Event)
	newProgress func(int64) io.WriteCloser
	listing     []plumbing.FileInfo
}

func NewDirectoryPackageBuilder(storage DirectoryPackageBuilderFileSystem, archive plumbing.ArchiveWriter, hasher hash.Hash, newProgress func(int64) io.WriteCloser, emit func(contracts.Event)) PackageBuilder {
	if newProgress == nil {
		newProgress = noopProgress
	}

	if emit == nil {
		emit = func(contracts.Event) {}
	}

	return &DirectoryPackageBuilder{
		storage:     storage,
		archive:     archive,
		hasher:      hasher,
		emit:        emit,
		newProgress: newProgress,
	}
}

func (this *DirectoryPackageBuilder) Build() error {
	var err error
	this.listing, err = this.storage.Listing()
	if err != nil {
		return err
	}

	if fileInfo, ok := this.fileOnly(); ok {
		if err = this.add(fileInfo, true); err != nil {
			return err
		}
	} else {
		for _, file := range this.listing {
			if err = this.add(file, false); err != nil {
				return err
			}
		}
	}

	return this.archive.Close()
}

func (this *DirectoryPackageBuilder) add(file plumbing.FileInfo, fileOnly bool) error {
	this.emit(contracts.Event{Type: contracts.EventProgress, Message: fmt.Sprintf("Adding %q to archive.", file.Path())})
	header, err := this.buildHeader(file, fileOnly)
	if err != nil {
		return err
	}
	if err = this.archive.WriteHeader(header); err != nil {
		return err
	}
	err = this.archiveContents(file, header.LinkName)
	if err != nil {
		return err
	}
	this.contents = append(this.contents, this.buildManifestEntry(file, header.LinkName))
	return err
}

func (this *DirectoryPackageBuilder) archiveContents(file plumbing.FileInfo, symlinkSourcePath string) error {
	if symlinkSourcePath != "" {
		_, _ = io.WriteString(this.hasher, symlinkSourcePath)
		return nil
	}
	progressWriter := this.newProgress(file.Size())
	defer closeResource(progressWriter)
	writer := io.MultiWriter(this.hasher, this.archive, progressWriter)
	reader, err := this.storage.Open(file.Path())
	if err != nil {
		return err
	}
	defer closeResource(reader)
	_, err = io.Copy(writer, reader)

	return err
}

func (this *DirectoryPackageBuilder) buildHeader(file plumbing.FileInfo, fileOnly bool) (header plumbing.ArchiveHeader, err error) {
	if fileOnly {
		header.Name = filepath.Base(file.Path())
	} else {
		header.Name = strings.TrimPrefix(file.Path(), this.storage.RootPath()+"/")
	}
	header.Size = file.Size()
	header.ModTime = file.ModTime()
	header.Executable = IsExecutable(file.Mode())
	if file.Symlink() == "" {
		return header, nil
	}

	if this.outOfBounds(file) {
		return header, this.symlinkOutOfBoundError(file)
	}
	header.LinkName, err = this.relativeLinkSourcePath(file)
	return header, err
}

func (this *DirectoryPackageBuilder) relativeLinkSourcePath(file plumbing.FileInfo) (string, error) {
	path := file.Symlink()
	if this.isAbsolute(path) {
		return filepath.Rel(filepath.Dir(file.Path()), path)
	}
	joined := filepath.Join(filepath.Dir(file.Path()), path)
	path = filepath.Clean(joined)
	return filepath.Rel(filepath.Dir(file.Path()), path)
}

func (this *DirectoryPackageBuilder) symlinkOutOfBoundError(file plumbing.FileInfo) error {
	return fmt.Errorf(
		"the file \"%s\" is a symlink that refers to \"%s\" which is outside of the configured root directory: \"%s\"",
		file.Path(),
		file.Symlink(),
		this.storage.RootPath())
}

func (this *DirectoryPackageBuilder) buildManifestEntry(file plumbing.FileInfo, symlinkSourcePath string) plumbing.ArchiveItem {
	defer this.hasher.Reset()
	var path string
	if _, ok := this.fileOnly(); ok == true {
		path = filepath.Base(file.Path())
	} else {
		path = strings.TrimPrefix(file.Path(), this.storage.RootPath()+"/")
	}

	return plumbing.ArchiveItem{
		Path:        path,
		Size:        this.determineFileSize(file, symlinkSourcePath),
		MD5Checksum: this.hasher.Sum(nil),
	}
}

func (this *DirectoryPackageBuilder) determineFileSize(file plumbing.FileInfo, symlinkSourcePath string) int64 {
	if symlinkSourcePath == "" {
		return file.Size()
	}
	return int64(len(symlinkSourcePath))
}

func (this *DirectoryPackageBuilder) Contents() []plumbing.ArchiveItem {
	return this.contents
}

func (this *DirectoryPackageBuilder) outOfBounds(info plumbing.FileInfo) bool {
	if this.isAbsolute(info.Symlink()) {
		return !strings.HasPrefix(info.Symlink(), this.storage.RootPath())
	}
	cleaned := filepath.Clean(filepath.Join(filepath.Dir(info.Path()), info.Symlink()))
	return !strings.HasPrefix(cleaned, this.storage.RootPath())
}

func (this *DirectoryPackageBuilder) isAbsolute(path string) bool {
	return strings.HasPrefix(path, "/")
}

func (this *DirectoryPackageBuilder) fileOnly() (plumbing.FileInfo, bool) {
	if len(this.listing) == 1 {
		if this.listing[0].Mode().IsRegular() {
			return this.listing[0], true
		}
	}
	return nil, false
}
