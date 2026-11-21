package packaging

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"github.com/smarty/satisfy/v2/internal/archive"
	"github.com/smarty/satisfy/v2/internal/model"
)

type PackageInstallerFileSystem interface {
	model.FileWriter
	model.Rooter
}

type PackageInstaller struct {
	downloader model.Downloader
	filesystem PackageInstallerFileSystem
	logger     *slog.Logger
	progress   model.ProgressFunc
}

func NewPackageInstaller(
	downloader model.Downloader,
	filesystem PackageInstallerFileSystem,
	logger *slog.Logger,
	progress model.ProgressFunc,
) *PackageInstaller {
	return &PackageInstaller{
		downloader: downloader,
		filesystem: filesystem,
		logger:     model.LoggerOrDiscard(logger),
		progress:   progress,
	}
}

func (this *PackageInstaller) DownloadManifest(ctx context.Context, remoteAddress url.URL) (manifest model.Manifest, err error) {
	body, err := this.downloader.Download(ctx, remoteAddress)
	if err != nil {
		return model.Manifest{}, err
	}

	defer closeResource(body)

	rawManifest, err := io.ReadAll(body)
	if err != nil {
		return model.Manifest{}, err
	}
	err = json.Unmarshal(rawManifest, &manifest)
	return manifest, err
}

// ManifestExists performs a HEAD request, sparing the cost of downloading the
// manifest body when only its presence matters.
func (this *PackageInstaller) ManifestExists(ctx context.Context, remoteAddress url.URL) (bool, error) {
	_, err := this.downloader.Size(ctx, remoteAddress)
	if model.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (this *PackageInstaller) InstallManifest(ctx context.Context, request model.InstallationRequest) (manifest model.Manifest, err error) {
	manifest, err = this.DownloadManifest(ctx, request.RemoteAddress)
	if err != nil {
		return model.Manifest{}, err
	}

	manifest.Name = request.PackageName
	manifest.Tags = nil // the tag listing only belongs in the remote root manifest
	rawManifest, err := json.Marshal(manifest, jsontext.WithIndent("  "))
	if err != nil {
		return model.Manifest{}, err
	}
	err = this.filesystem.WriteFile(ComposeManifestPath(request.LocalPath, manifest.Name), rawManifest)
	if err != nil {
		return model.Manifest{}, err
	}
	return manifest, nil
}

func (this *PackageInstaller) InstallPackage(ctx context.Context, manifest model.Manifest, request model.InstallationRequest) error {
	body, err := this.downloader.Download(ctx, request.RemoteAddress)
	if err != nil {
		return err
	}
	resuming := newResumingReader(ctx, this.downloader, request.RemoteAddress, body, manifest.Archive.Size)
	defer closeResource(resuming)
	checksumReader := NewHashReader(resuming, md5.New())

	reader, decompressor, err := OpenArchiveReader(checksumReader, manifest.Archive.CompressionAlgorithm, this.progress)
	if err != nil {
		return err
	}
	root, err := this.filesystem.OpenRoot(request.LocalPath)
	if err != nil {
		return errors.Join(err, decompressor.Close())
	}
	defer closeResource(root)
	names, err := this.extractArchive(reader, root, request, len(manifest.Archive.Contents))
	closeResource(decompressor)
	if err != nil {
		return errors.Join(err, revertFileSystem(root, names))
	}
	// The archive reader stops at its end-of-archive marker, which can leave
	// the end of the compressed stream unread; the checksum covers all of it.
	if _, err = io.Copy(io.Discard, checksumReader); err != nil {
		return errors.Join(err, revertFileSystem(root, names))
	}
	actualChecksum := checksumReader.Sum(nil)
	if !bytes.Equal(actualChecksum, manifest.Archive.MD5Checksum) {
		err = fmt.Errorf("checksum mismatch: actual [%x] != expected [%x]", actualChecksum, manifest.Archive.MD5Checksum)
		return errors.Join(err, revertFileSystem(root, names))
	}

	return nil
}

// extractArchive writes each archive entry within root and returns the names
// of the entries it wrote, relative to root.
func (this *PackageInstaller) extractArchive(reader ArchiveReader, root model.RootedFileSystem, request model.InstallationRequest, itemCount int) (names []string, err error) {
	for i := 0; ; i++ {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return names, err
		}
		if err = checkArchiveEntry(header); err != nil {
			return names, err
		}
		name := filepath.Clean(header.Name)
		pathItem := filepath.Join(request.LocalPath, name)
		names = append(names, name)
		this.logger.Debug("extracting archive item",
			"item", i+1, "of", itemCount, "name", header.Name, "size", archive.HumanFileSize(header.Size), "path", pathItem)

		if header.Typeflag == tar.TypeSymlink {
			if err = root.CreateSymlink(header.Linkname, name); err != nil {
				return names, err
			}
		} else {
			writer, err := root.Create(name)
			if err != nil {
				return names, err
			}
			progressReader := archive.NewProgressCounter(this.progress, model.ProgressExtracting, pathItem, header.Size)
			multiWriter := io.MultiWriter(writer, progressReader)
			_, err = io.Copy(multiWriter, reader)
			err = errors.Join(err, writer.Close())
			_ = progressReader.Close()
			if err != nil {
				return names, err
			}
			if !model.IsExecutable(os.FileMode(header.Mode)) {
				continue
			}
			if err = root.Chmod(name, 0755); err != nil {
				return names, err
			}
		}
	}
	return names, nil
}

// checkArchiveEntry refuses entries that would be written outside the
// installation directory, or symlinks that point outside it.
func checkArchiveEntry(header *tar.Header) error {
	if !filepath.IsLocal(header.Name) {
		return fmt.Errorf("archive entry %q is outside the installation directory", header.Name)
	}
	if header.Typeflag == tar.TypeSymlink && !isLocalSymlink(header.Name, header.Linkname) {
		return fmt.Errorf("archive symlink %q -> %q points outside the installation directory", header.Name, header.Linkname)
	}
	return nil
}

// isLocalSymlink reports whether a symlink at name pointing to target stays
// within the directory that contains name.
func isLocalSymlink(name, target string) bool {
	return !filepath.IsAbs(target) && filepath.IsLocal(filepath.Join(filepath.Dir(name), target))
}

func revertFileSystem(root model.RootedFileSystem, names []string) (err error) {
	for _, name := range names {
		err = errors.Join(err, ignoreNotExist(root.Delete(name)))
	}
	return err
}

func ignoreNotExist(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func ComposeManifestPath(localPath, packageName string) string {
	cleanPackageName := strings.ReplaceAll(packageName, "/", "___")
	fileName := fmt.Sprintf("manifest_%s.json", cleanPackageName)
	return filepath.Join(localPath, fileName)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var decompressors = map[string]func(source io.Reader, progress model.ProgressFunc) (io.ReadCloser, error){
	"zstd": newZStdReader,
	"zip":  newZipReader,
	"gzip": newGZipReader,
}

func newZStdReader(source io.Reader, _ model.ProgressFunc) (io.ReadCloser, error) {
	reader, err := zstd.NewReader(source)
	if err != nil {
		return nil, err
	}
	return reader.IOReadCloser(), nil
}
func newZipReader(source io.Reader, progress model.ProgressFunc) (io.ReadCloser, error) {
	return archive.NewZipArchiveReader(source, progress)
}
func newGZipReader(source io.Reader, _ model.ProgressFunc) (io.ReadCloser, error) {
	return gzip.NewReader(source)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type ArchiveReader interface {
	Next() (*tar.Header, error)
	io.Reader
}

// OpenArchiveReader decompresses source with the named algorithm and returns
// a reader of the archive's entries, along with the decompressor, which the
// caller must close.
func OpenArchiveReader(source io.Reader, algorithm string, progress model.ProgressFunc) (ArchiveReader, io.Closer, error) {
	factory, found := decompressors[algorithm]
	if !found {
		return nil, nil, fmt.Errorf("unsupported compression algorithm: %q", algorithm)
	}
	decompressor, err := factory(source, progress)
	if err != nil {
		return nil, nil, err
	}
	if reader, ok := decompressor.(ArchiveReader); ok {
		return reader, decompressor, nil // zip
	}
	return tar.NewReader(decompressor), decompressor, nil
}

func closeResource(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}
