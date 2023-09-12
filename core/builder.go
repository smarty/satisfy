package core

import (
	"fmt"
	"hash"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/smarty/satisfy/contracts"
)

type PackageBuilderFileSystem interface {
	contracts.PathLister
	contracts.FileOpener
	contracts.RootPath
}

type PackageBuilder interface {
	Build() error
	Contents() []contracts.ArchiveItem
}

type FilePackageBuilder struct {
	sourceFile string
	writer     io.Writer
	hasher     hash.Hash
	contents   []contracts.ArchiveItem
}

func NewFilePackageBuilder(sourceFile string, writer io.Writer, hasher hash.Hash) PackageBuilder {
	return &FilePackageBuilder{
		sourceFile: sourceFile,
		writer:     writer,
		hasher:     hasher,
	}
}

func (this *FilePackageBuilder) Build() error {
	file, err := os.Open(this.sourceFile)
	defer func() { _ = file.Close() }()
	if err != nil {
		return err
	}
	_, err = io.Copy(this.writer, file)
	if err != nil {
		return err
	}
	md5Sum := this.hasher.Sum(nil)

	fileInfo, err := os.Stat(file.Name())
	if err != nil {
		log.Fatal(err)
	}

	archiveItem := contracts.ArchiveItem{
		Path:        filepath.Base(this.sourceFile),
		Size:        fileInfo.Size(),
		MD5Checksum: md5Sum,
	}
	this.contents = []contracts.ArchiveItem{archiveItem}
	return err
}

func (this *FilePackageBuilder) Contents() []contracts.ArchiveItem {
	return this.contents
}

type DirectoryPackageBuilder struct {
	storage  PackageBuilderFileSystem
	archive  contracts.ArchiveWriter
	hasher   hash.Hash
	contents []contracts.ArchiveItem
}

func NewDirectoryPackageBuilder(storage PackageBuilderFileSystem, archive contracts.ArchiveWriter, hasher hash.Hash) PackageBuilder {
	return &DirectoryPackageBuilder{
		storage: storage,
		archive: archive,
		hasher:  hasher,
	}
}

func (this *DirectoryPackageBuilder) Build() error {
	for _, file := range this.storage.Listing() {
		err := this.add(file)
		if err != nil {
			return err
		}
	}
	return this.archive.Close()
}

func (this *DirectoryPackageBuilder) add(file contracts.FileInfo) error {
	log.Printf("Adding \"%s\" to archive.", file.Path())
	header, err := this.buildHeader(file)
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

func (this *DirectoryPackageBuilder) archiveContents(file contracts.FileInfo, symlinkSourcePath string) error {
	if symlinkSourcePath != "" {
		_, _ = io.WriteString(this.hasher, symlinkSourcePath)
		return nil
	}
	progressWriter := newArchiveProgressCounter(file.Size(), func(archived, total string) {
		fmt.Printf("\033[2K\rArchived %s of %s.", archived, total)
	})
	defer func() {
		fmt.Printf("\n")
	}()
	defer closeResource(progressWriter)
	writer := io.MultiWriter(this.hasher, this.archive, progressWriter)
	reader := this.storage.Open(file.Path())
	defer closeResource(reader)
	_, err := io.Copy(writer, reader)

	return err
}

func (this *DirectoryPackageBuilder) buildHeader(file contracts.FileInfo) (header contracts.ArchiveHeader, err error) {
	header.Name = strings.TrimPrefix(file.Path(), this.storage.RootPath()+"/")
	header.Size = file.Size()
	header.ModTime = file.ModTime()
	header.Executable = contracts.IsExecutable(file.Mode())
	if file.Symlink() == "" {
		return header, nil
	}

	if this.outOfBounds(file) {
		return header, this.symlinkOutOfBoundError(file)
	}
	header.LinkName, err = this.relativeLinkSourcePath(file)
	return header, err
}

func (this *DirectoryPackageBuilder) relativeLinkSourcePath(file contracts.FileInfo) (string, error) {
	path := file.Symlink()
	if this.isAbsolute(path) {
		return filepath.Rel(filepath.Dir(file.Path()), path)
	}
	joined := filepath.Join(filepath.Dir(file.Path()), path)
	path = filepath.Clean(joined)
	return filepath.Rel(filepath.Dir(file.Path()), path)
}

func (this *DirectoryPackageBuilder) symlinkOutOfBoundError(file contracts.FileInfo) error {
	return fmt.Errorf(
		"the file \"%s\" is a symlink that refers to \"%s\" which is outside of the configured root directory: \"%s\"",
		file.Path(),
		file.Symlink(),
		this.storage.RootPath())
}

func (this *DirectoryPackageBuilder) buildManifestEntry(file contracts.FileInfo, symlinkSourcePath string) contracts.ArchiveItem {
	defer this.hasher.Reset()
	return contracts.ArchiveItem{
		Path:        strings.TrimPrefix(file.Path(), this.storage.RootPath()+"/"),
		Size:        this.determineFileSize(file, symlinkSourcePath),
		MD5Checksum: this.hasher.Sum(nil),
	}
}

func (this *DirectoryPackageBuilder) determineFileSize(file contracts.FileInfo, symlinkSourcePath string) int64 {
	if symlinkSourcePath == "" {
		return file.Size()
	}
	return int64(len(symlinkSourcePath))
}

func (this *DirectoryPackageBuilder) Contents() []contracts.ArchiveItem {
	return this.contents
}

func (this *DirectoryPackageBuilder) outOfBounds(info contracts.FileInfo) bool {
	if this.isAbsolute(info.Symlink()) {
		return !strings.HasPrefix(info.Symlink(), this.storage.RootPath())
	}
	cleaned := filepath.Clean(filepath.Join(filepath.Dir(info.Path()), info.Symlink()))
	return !strings.HasPrefix(cleaned, this.storage.RootPath())
}

func (this *DirectoryPackageBuilder) isAbsolute(path string) bool {
	return strings.HasPrefix(path, "/")
}
