package core

import (
	"context"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/smarty/satisfy/v2/cmd/archive_progress"
	"github.com/smarty/satisfy/v2/contracts"
)

type PackageBuilder interface {
	Build(ctx context.Context) error
	Contents() []contracts.ArchiveItem
}

type DirectoryPackageBuilderFileSystem interface {
	contracts.PathLister
	contracts.FileOpener
	contracts.RootPath
}

type DirectoryPackageBuilder struct {
	storage  DirectoryPackageBuilderFileSystem
	archive  contracts.ArchiveWriter
	hasher   hash.Hash
	contents []contracts.ArchiveItem
	listing  []contracts.FileInfo
	logger   *slog.Logger
	progress contracts.ProgressFunc
}

func NewDirectoryPackageBuilder(
	storage DirectoryPackageBuilderFileSystem,
	archive contracts.ArchiveWriter,
	hasher hash.Hash,
	logger *slog.Logger,
	progress contracts.ProgressFunc,
) PackageBuilder {
	return &DirectoryPackageBuilder{
		storage:  storage,
		archive:  archive,
		hasher:   hasher,
		logger:   contracts.LoggerOrDiscard(logger),
		progress: progress,
	}
}

// Build archives the listing. It stops with ctx's error once ctx is done,
// checking between files and while copying each file.
func (this *DirectoryPackageBuilder) Build(ctx context.Context) error {
	listing, err := this.storage.Listing()
	if err != nil {
		return err
	}
	this.listing = listing
	if fileInfo, ok := this.fileOnly(); ok {
		err = this.add(ctx, fileInfo, true)
		if err != nil {
			return err
		}
	} else {
		for _, file := range listing {
			if err = ctx.Err(); err != nil {
				return err
			}
			err = this.add(ctx, file, false)
			if err != nil {
				return err
			}
		}
	}
	return this.archive.Close()
}

func (this *DirectoryPackageBuilder) add(ctx context.Context, file contracts.FileInfo, fileOnly bool) error {
	this.logger.Info("adding file to archive", "path", file.Path())
	header, err := this.buildHeader(file, fileOnly)
	if err != nil {
		return err
	}
	err = this.archive.WriteHeader(header)
	if err != nil {
		return err
	}
	err = this.archiveContents(ctx, file, header.LinkName)
	if err != nil {
		return err
	}
	this.contents = append(this.contents, this.buildManifestEntry(file, header.LinkName))
	return err
}

func (this *DirectoryPackageBuilder) archiveContents(ctx context.Context, file contracts.FileInfo, symlinkSourcePath string) error {
	if symlinkSourcePath != "" {
		_, _ = io.WriteString(this.hasher, symlinkSourcePath)
		return nil
	}
	progressWriter := archive_progress.NewArchiveProgressCounter(file.Size(), func(written, total int64, done bool) {
		this.progress.Report(contracts.Progress{
			Phase: contracts.ProgressArchiving, Name: file.Path(), Written: written, Total: total, Done: done,
		})
	})
	defer closeResource(progressWriter)
	reader, err := this.storage.Open(file.Path())
	if err != nil {
		return err
	}
	defer closeResource(reader)
	// Hashing runs on its own goroutine so it overlaps with archiving and
	// compression. It starts only after Open succeeds, and the
	// deferred Close both stops the goroutine and finishes hashing before
	// buildManifestEntry reads the sum.
	hasher := NewAsyncHashWriter(this.hasher)
	defer closeResource(hasher)
	writer := io.MultiWriter(hasher, this.archive, progressWriter)
	_, err = io.Copy(writer, contextReader{ctx: ctx, reader: reader})
	return err
}

// contextReader fails with ctx's error once ctx is done, so that copying a
// large file stops promptly when the operation is cancelled.
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (this contextReader) Read(buffer []byte) (int, error) {
	if err := this.ctx.Err(); err != nil {
		return 0, err
	}
	return this.reader.Read(buffer)
}

func (this *DirectoryPackageBuilder) buildHeader(file contracts.FileInfo, fileOnly bool) (header contracts.ArchiveHeader, err error) {
	if fileOnly {
		header.Name = filepath.Base(file.Path())
	} else {
		header.Name = strings.TrimPrefix(file.Path(), this.storage.RootPath()+"/")
	}
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
	var path string
	if _, ok := this.fileOnly(); ok {
		path = filepath.Base(file.Path())
	} else {
		path = strings.TrimPrefix(file.Path(), this.storage.RootPath()+"/")
	}
	return contracts.ArchiveItem{
		Path:        path,
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

func (this *DirectoryPackageBuilder) fileOnly() (contracts.FileInfo, bool) {
	if len(this.listing) == 1 && this.listing[0].Mode().IsRegular() {
		return this.listing[0], true
	}
	return nil, false
}
