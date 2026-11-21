package packaging

import (
	"context"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/smarty/satisfy/v2/internal/archive"
	"github.com/smarty/satisfy/v2/internal/model"
)

type DirectoryPackageBuilderFileSystem interface {
	model.PathLister
	model.FileOpener
	model.RootPath
}

type DirectoryPackageBuilder struct {
	storage  DirectoryPackageBuilderFileSystem
	archive  model.ArchiveWriter
	hasher   hash.Hash
	contents []model.ArchiveItem
	listing  []model.FileInfo
	logger   *slog.Logger
	progress model.ProgressFunc
	exclude  func(relativePath string) bool
}

func NewDirectoryPackageBuilder(
	storage DirectoryPackageBuilderFileSystem,
	archive model.ArchiveWriter,
	hasher hash.Hash,
	logger *slog.Logger,
	progress model.ProgressFunc,
) *DirectoryPackageBuilder {
	return &DirectoryPackageBuilder{
		storage:  storage,
		archive:  archive,
		hasher:   hasher,
		logger:   model.LoggerOrDiscard(logger),
		progress: progress,
	}
}

// WithExclusion leaves out of a directory's archive every file for which
// exclude returns true, given the file's path relative to the directory. It
// has no effect when the source is a single file.
func (this *DirectoryPackageBuilder) WithExclusion(exclude func(relativePath string) bool) *DirectoryPackageBuilder {
	this.exclude = exclude
	return this
}

// Build archives the listing. It stops with ctx's error once ctx is done,
// checking between files and while copying each file. A file name that is not
// valid UTF-8 (which the JSON manifest cannot hold) fails the build before
// anything is archived.
func (this *DirectoryPackageBuilder) Build(ctx context.Context) error {
	listing, err := this.storage.Listing()
	if err != nil {
		return err
	}
	this.listing = listing
	if fileInfo, ok := this.fileOnly(); ok {
		if err = checkName(filepath.Base(fileInfo.Path()), fileInfo.Path()); err != nil {
			return err
		}
		err = this.add(ctx, fileInfo, true)
		if err != nil {
			return err
		}
	} else {
		included, err := this.included()
		if err != nil {
			return err
		}
		for _, file := range included {
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

// included returns the files of the listing that are not excluded, checking
// that their names can be recorded in the manifest.
func (this *DirectoryPackageBuilder) included() (included []model.FileInfo, err error) {
	for _, file := range this.listing {
		relativePath := this.relativePath(file)
		if this.exclude != nil && this.exclude(relativePath) {
			this.logger.Info("excluding file from archive", "path", file.Path())
			continue
		}
		if err = checkName(relativePath, file.Path()); err != nil {
			return nil, err
		}
		included = append(included, file)
	}
	return included, nil
}

func checkName(name, fullPath string) error {
	if utf8.ValidString(name) {
		return nil
	}
	return fmt.Errorf("the file %q cannot be archived: its name is not valid UTF-8", fullPath)
}

func (this *DirectoryPackageBuilder) relativePath(file model.FileInfo) string {
	return strings.TrimPrefix(file.Path(), this.storage.RootPath()+"/")
}

func (this *DirectoryPackageBuilder) add(ctx context.Context, file model.FileInfo, fileOnly bool) error {
	this.logger.Debug("adding file to archive", "path", file.Path())
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

func (this *DirectoryPackageBuilder) archiveContents(ctx context.Context, file model.FileInfo, symlinkSourcePath string) error {
	if symlinkSourcePath != "" {
		_, _ = io.WriteString(this.hasher, symlinkSourcePath)
		return nil
	}
	progressWriter := archive.NewProgressCounter(this.progress, model.ProgressArchiving, file.Path(), file.Size())
	defer closeResource(progressWriter)
	reader, err := this.storage.Open(file.Path())
	if err != nil {
		return err
	}
	defer closeResource(reader)
	hasher := io.Writer(this.hasher)
	if file.Size() >= asyncHashThreshold {
		// For large files, hashing on its own goroutine overlaps with archiving
		// and compression. The deferred Close both stops the goroutine and
		// finishes hashing before buildManifestEntry reads the sum.
		async := NewAsyncHashWriter(this.hasher)
		defer closeResource(async)
		hasher = async
	}
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

// asyncHashThreshold is the file size from which hashing on a separate
// goroutine pays off. Below it, handing each write to the hashing goroutine
// costs more than it saves (measured with gzip and zstd at their default
// levels: up to ~40% slower for small files; ~50% faster for 32 MiB files
// with gzip, which compresses on the calling goroutine).
const asyncHashThreshold = 16 << 20

func (this *DirectoryPackageBuilder) buildHeader(file model.FileInfo, fileOnly bool) (header model.ArchiveHeader, err error) {
	if fileOnly {
		header.Name = filepath.Base(file.Path())
	} else {
		header.Name = this.relativePath(file)
	}
	header.Size = file.Size()
	header.ModTime = file.ModTime()
	header.Executable = model.IsExecutable(file.Mode())
	if file.Symlink() == "" {
		return header, nil
	}

	if this.outOfBounds(file) {
		return header, this.symlinkOutOfBoundError(file)
	}
	header.LinkName, err = this.relativeLinkSourcePath(file)
	return header, err
}

func (this *DirectoryPackageBuilder) relativeLinkSourcePath(file model.FileInfo) (string, error) {
	path := file.Symlink()
	if this.isAbsolute(path) {
		return filepath.Rel(filepath.Dir(file.Path()), path)
	}
	joined := filepath.Join(filepath.Dir(file.Path()), path)
	path = filepath.Clean(joined)
	return filepath.Rel(filepath.Dir(file.Path()), path)
}

func (this *DirectoryPackageBuilder) symlinkOutOfBoundError(file model.FileInfo) error {
	return fmt.Errorf(
		"the file \"%s\" is a symlink that refers to \"%s\" which is outside of the configured root directory: \"%s\"",
		file.Path(),
		file.Symlink(),
		this.storage.RootPath())
}

func (this *DirectoryPackageBuilder) buildManifestEntry(file model.FileInfo, symlinkSourcePath string) model.ArchiveItem {
	defer this.hasher.Reset()
	var path string
	if _, ok := this.fileOnly(); ok {
		path = filepath.Base(file.Path())
	} else {
		path = this.relativePath(file)
	}
	return model.ArchiveItem{
		Path:        path,
		Size:        this.determineFileSize(file, symlinkSourcePath),
		MD5Checksum: this.hasher.Sum(nil),
	}
}

func (this *DirectoryPackageBuilder) determineFileSize(file model.FileInfo, symlinkSourcePath string) int64 {
	if symlinkSourcePath == "" {
		return file.Size()
	}
	return int64(len(symlinkSourcePath))
}

func (this *DirectoryPackageBuilder) Contents() []model.ArchiveItem {
	return this.contents
}

// outOfBounds reports whether a symlink points outside the root directory. A
// bare prefix comparison would accept "/root-other/x" for root "/root".
func (this *DirectoryPackageBuilder) outOfBounds(info model.FileInfo) bool {
	target := info.Symlink()
	if !this.isAbsolute(target) {
		target = filepath.Join(filepath.Dir(info.Path()), target)
	}
	relative, err := filepath.Rel(this.storage.RootPath(), filepath.Clean(target))
	return err != nil || !filepath.IsLocal(relative)
}

func (this *DirectoryPackageBuilder) isAbsolute(path string) bool {
	return strings.HasPrefix(path, "/")
}

func (this *DirectoryPackageBuilder) fileOnly() (model.FileInfo, bool) {
	if len(this.listing) == 1 && this.listing[0].Mode().IsRegular() {
		return this.listing[0], true
	}
	return nil, false
}
