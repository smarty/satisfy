package satisfy

import (
	"context"
	"crypto/md5"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"github.com/smarty/satisfy/v2/internal/archive"
	"github.com/smarty/satisfy/v2/internal/fsys"
	"github.com/smarty/satisfy/v2/internal/model"
	"github.com/smarty/satisfy/v2/internal/packaging"
)

// UploadRequest describes a package version to upload.
type UploadRequest struct {
	Package PackageConfig

	// Overwrite replaces the package version when it already exists instead
	// of failing with ErrPackageExists.
	Overwrite bool

	// ExcludeInstalledManifests leaves out of a directory's archive the
	// manifest_*.json files that Install writes (at any depth), so a
	// directory that packages were installed into can be uploaded as is. It
	// has no effect when the source is a single file.
	ExcludeInstalledManifests bool
}

// Upload archives the package's source, uploads the archive and its manifest,
// and points the package's root manifest (and any configured tags) at the new
// version. It fails with ErrPackageExists when the version already exists,
// unless request.Overwrite is set.
func (this *Client) Upload(ctx context.Context, request UploadRequest) (Manifest, error) {
	if err := ValidatePackageConfig(request.Package); err != nil {
		return Manifest{}, err
	}
	pkg := request.Package.internal()

	if !request.Overwrite {
		exists, err := this.Exists(ctx, PackageRef{RemoteAddress: request.Package.RemoteAddress, Name: pkg.PackageName}, pkg.PackageVersion)
		if err != nil {
			return Manifest{}, fmt.Errorf("could not check for an existing package: %w", err)
		}
		if exists {
			return Manifest{}, fmt.Errorf("%s @ %s: %w", pkg.PackageName, pkg.PackageVersion, ErrPackageExists)
		}
	}

	built, err := this.buildArchive(ctx, pkg, request.ExcludeInstalledManifests)
	if err != nil {
		return Manifest{}, err
	}
	defer func() { _ = os.Remove(built.path) }()

	manifest := model.Manifest{
		Name:    pkg.PackageName,
		Version: pkg.PackageVersion,
		Archive: model.Archive{
			Filename:             model.RemoteArchiveFilename,
			Size:                 uint64(built.size),
			MD5Checksum:          built.checksum,
			Contents:             built.contents,
			CompressionAlgorithm: pkg.CompressionAlgorithm,
		},
	}
	if this.logger.Enabled(ctx, slog.LevelDebug) {
		raw, _ := json.Marshal(manifest, jsontext.WithIndent("  "))
		this.logger.Debug("built manifest", "manifest", string(raw))
	}

	this.logger.Info("uploading the archive", "package", pkg.PackageName, "version", pkg.PackageVersion)
	if err = this.uploadArchive(ctx, pkg, manifest, built.path); err != nil {
		return Manifest{}, err
	}

	this.logger.Info("uploading the manifest", "package", pkg.PackageName, "version", pkg.PackageVersion)
	if err = this.uploadManifest(ctx, manifest, pkg.ComposeRemoteAddress(model.RemoteManifestFilename), nil); err != nil {
		return Manifest{}, err
	}
	err = this.updateRootManifest(ctx, pkg.ComposeLatestManifestRemoteAddress(), func(existing model.Manifest, _ bool) (model.Manifest, error) {
		return this.buildRootManifest(pkg, manifest, existing), nil
	})
	if err != nil {
		return Manifest{}, err
	}
	return publicManifest(manifest), nil
}

// ValidatePackageConfig reports incomplete or inconsistent upload input with
// an error matching ErrInvalidConfig. Upload calls it before any I/O.
func ValidatePackageConfig(pkg PackageConfig) error {
	switch {
	case pkg.PackageName == "":
		return fmt.Errorf("%w: package name is required", ErrInvalidConfig)
	case pkg.PackageVersion == "":
		return fmt.Errorf("%w: package version is required", ErrInvalidConfig)
	case pkg.RemoteAddress.Host == "":
		return fmt.Errorf("%w: remote address is required", ErrInvalidConfig)
	}
	switch sources := countNonBlank(pkg.SourcePath, pkg.SourceDirectory, pkg.SourceFile); {
	case sources == 0:
		return fmt.Errorf("%w: source path, source directory or source file is required", ErrInvalidConfig)
	case sources > 1:
		return fmt.Errorf("%w: only one of source path, source directory and source file may be given", ErrInvalidConfig)
	}
	if _, found := compression[pkg.CompressionAlgorithm]; !found {
		return fmt.Errorf("%w: unsupported compression algorithm: %q", ErrInvalidConfig, pkg.CompressionAlgorithm)
	}
	for _, tag := range pkg.Tags {
		if tag == "" || tag == "latest" {
			return fmt.Errorf("%w: invalid tag name: %q", ErrInvalidConfig, tag)
		}
	}
	return nil
}

// isInstalledManifest reports whether the file at relativePath is named like
// the manifests Install writes.
func isInstalledManifest(relativePath string) bool {
	matched, _ := path.Match("manifest_*.json", path.Base(filepath.ToSlash(relativePath)))
	return matched
}

func sourcePath(pkg model.PackageConfig) string {
	switch {
	case pkg.SourcePath != "":
		return pkg.SourcePath
	case pkg.SourceDirectory != "":
		return pkg.SourceDirectory
	default:
		return pkg.SourceFile
	}
}

func countNonBlank(values ...string) (count int) {
	for _, value := range values {
		if value != "" {
			count++
		}
	}
	return count
}

type builtArchive struct {
	path     string
	size     int64
	checksum []byte
	contents []model.ArchiveItem
}

func (this *Client) buildArchive(ctx context.Context, pkg model.PackageConfig, excludeInstalledManifests bool) (built builtArchive, err error) {
	file, err := os.CreateTemp("", "satisfy-archive-*")
	if err != nil {
		return builtArchive{}, err
	}
	built.path = file.Name()
	defer func() {
		err = errors.Join(err, file.Close())
		if err != nil {
			_ = os.Remove(built.path)
		}
	}()

	hasher := md5.New()
	counter := &byteCounter{}
	compressor, err := newCompressor(pkg.CompressionAlgorithm, pkg.CompressionLevel, io.MultiWriter(hasher, file, counter))
	if err != nil {
		return builtArchive{}, err
	}

	builder := packaging.NewDirectoryPackageBuilder(
		fsys.NewDiskFileSystem(sourcePath(pkg)),
		archive.NewSwitchArchiveWriter(compressor),
		md5.New(),
		this.logger,
		this.progress,
	)
	if excludeInstalledManifests {
		builder.WithExclusion(isInstalledManifest)
	}
	if err = builder.Build(ctx); err != nil {
		_ = compressor.Close()
		return builtArchive{}, err
	}
	if err = compressor.Close(); err != nil {
		return builtArchive{}, err
	}
	built.size = counter.count
	built.checksum = hasher.Sum(nil)
	built.contents = builder.Contents()
	return built, nil
}

func (this *Client) uploadArchive(ctx context.Context, pkg model.PackageConfig, manifest model.Manifest, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return this.storage.Upload(ctx, model.UploadRequest{
		RemoteAddress: pkg.ComposeRemoteAddress(model.RemoteArchiveFilename),
		Body:          file,
		Size:          int64(manifest.Archive.Size),
		ContentType:   contentType[manifest.Archive.CompressionAlgorithm],
		Checksum:      manifest.Archive.MD5Checksum,
	})
}

// buildRootManifest carries the tag listing forward from the existing root
// manifest (empty on the first upload of a package) and points any tags named
// in the package config at the version being uploaded. The versioned manifest
// never includes tags.
func (this *Client) buildRootManifest(pkg model.PackageConfig, manifest, existing model.Manifest) model.Manifest {
	rootManifest := manifest
	rootManifest.Tags = model.MergeTags(existing.Tags, pkg.Tags, pkg.PackageVersion)
	for _, name := range pkg.Tags {
		this.logger.Info("tagging version", "version", pkg.PackageVersion, "tag", name)
	}
	return rootManifest
}

func newCompressor(algorithm string, level int, writer io.Writer) (io.WriteCloser, error) {
	factory, found := compression[algorithm]
	if !found {
		return nil, fmt.Errorf("%w: unsupported compression algorithm: %q", ErrInvalidConfig, algorithm)
	}
	return factory(writer, level)
}

var compression = map[string]func(_ io.Writer, level int) (io.WriteCloser, error){
	"zstd": func(writer io.Writer, level int) (io.WriteCloser, error) {
		return archive.NewParallelZstdWriter(writer, zstd.EncoderLevelFromZstd(level), archive.DefaultParallelZstdChunkSize)
	},
	"gzip": func(writer io.Writer, level int) (io.WriteCloser, error) {
		return gzip.NewWriterLevel(writer, level)
	},
	"zip": func(writer io.Writer, level int) (io.WriteCloser, error) {
		return archive.NewZipArchiveWriter(writer, level), nil
	},
}

var contentType = map[string]string{
	"zstd": "application/zstd",
	"gzip": "application/gzip",
	"zip":  "application/zip",
}

type byteCounter struct{ count int64 }

func (this *byteCounter) Write(p []byte) (int, error) {
	this.count += int64(len(p))
	return len(p), nil
}
