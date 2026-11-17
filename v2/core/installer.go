package core

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"github.com/smarty/satisfy/v2/cmd/archive_progress"
	"github.com/smarty/satisfy/v2/contracts"
	"github.com/smarty/satisfy/v2/shell"
)

type PackageInstallerFileSystem interface {
	contracts.FileCreator
	contracts.FileWriter
	contracts.Deleter
	contracts.SymlinkCreator
	contracts.Chmod
}

type PackageInstaller struct {
	downloader   contracts.Downloader
	filesystem   PackageInstallerFileSystem
	showProgress bool
}

func NewPackageInstaller(downloader contracts.Downloader, filesystem PackageInstallerFileSystem, showProgress bool) *PackageInstaller {
	return &PackageInstaller{downloader: downloader, filesystem: filesystem, showProgress: showProgress}
}

func (this *PackageInstaller) DownloadManifest(ctx context.Context, remoteAddress url.URL) (manifest contracts.Manifest, err error) {
	body, err := this.downloader.Download(ctx, remoteAddress)
	if err != nil {
		return contracts.Manifest{}, err
	}

	defer closeResource(body)

	rawManifest, err := io.ReadAll(body)
	if err != nil {
		return contracts.Manifest{}, err
	}
	err = json.Unmarshal(rawManifest, &manifest)
	return manifest, err
}

// ManifestExists performs a HEAD request, sparing the cost of downloading the
// manifest body when only its presence matters.
func (this *PackageInstaller) ManifestExists(ctx context.Context, remoteAddress url.URL) (bool, error) {
	_, err := this.downloader.Size(ctx, remoteAddress)
	if contracts.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (this *PackageInstaller) InstallManifest(ctx context.Context, request contracts.InstallationRequest) (manifest contracts.Manifest, err error) {
	manifest, err = this.DownloadManifest(ctx, request.RemoteAddress)
	if err != nil {
		return contracts.Manifest{}, err
	}

	manifest.Name = request.PackageName
	manifest.Tags = nil // the tag listing only belongs in the remote root manifest
	rawManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return contracts.Manifest{}, err
	}
	err = this.filesystem.WriteFile(ComposeManifestPath(request.LocalPath, manifest.Name), rawManifest)
	if err != nil {
		return contracts.Manifest{}, err
	}
	return manifest, nil
}

func (this *PackageInstaller) InstallPackage(ctx context.Context, manifest contracts.Manifest, request contracts.InstallationRequest) error {
	body, err := this.downloader.Download(ctx, request.RemoteAddress)
	if err != nil {
		return err
	}

	defer closeResource(body)
	checksumReader := NewHashReader(body, md5.New())

	factory, found := decompressors[manifest.Archive.CompressionAlgorithm]
	if !found {
		return errors.New("invalid compression algorithm")
	}
	decompressor, err := factory(checksumReader, this.showProgress)
	if err != nil {
		return err
	}
	paths, err := this.extractArchive(decompressor, request, len(manifest.Archive.Contents))
	if err != nil {
		return errors.Join(err, this.revertFileSystem(paths))
	}
	actualChecksum := checksumReader.Sum(nil)
	if !bytes.Equal(actualChecksum, manifest.Archive.MD5Checksum) {
		err = fmt.Errorf("checksum mismatch: actual [%x] != expected [%x]", actualChecksum, manifest.Archive.MD5Checksum)
		return errors.Join(err, this.revertFileSystem(paths))
	}

	return nil
}

func (this *PackageInstaller) extractArchive(decompressor io.ReadCloser, request contracts.InstallationRequest, itemCount int) (paths []string, err error) {
	defer closeResource(decompressor)
	var reader ArchiveReader
	if archiveReader, ok := decompressor.(ArchiveReader); ok {
		reader = archiveReader
	} else {
		reader = archiveFormats[""](decompressor)
	}

	for i := 0; ; i++ {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return paths, err
		}
		pathItem := filepath.Join(request.LocalPath, header.Name)
		paths = append(paths, pathItem)
		log.Printf("Extracting archive item [%d/%d] \"%s\" [%s] to \"%s\".",
			i+1, itemCount, header.Name, byteCountToString(header.Size), pathItem)

		if header.Typeflag == tar.TypeSymlink {
			if err = this.filesystem.CreateSymlink(header.Linkname, pathItem); err != nil {
				return paths, err
			}
		} else {
			writer, err := this.filesystem.Create(pathItem)
			if err != nil {
				return paths, err
			}
			progressReader := archive_progress.NewArchiveProgressCounter(header.Size, func(archived, total string, done bool) {
				if this.showProgress {
					if done {
						fmt.Printf("\nDone extracting %s.\n", archived)
					} else {
						fmt.Printf("\033[2K\rExtracted %s of %s.", archived, total)
					}
				}
			})
			multiWriter := io.MultiWriter(writer, progressReader)
			_, err = io.Copy(multiWriter, reader)
			err = errors.Join(err, writer.Close())
			_ = progressReader.Close()
			if err != nil {
				return paths, err
			}
			if !contracts.IsExecutable(os.FileMode(header.Mode)) {
				continue
			}
			if err = this.filesystem.Chmod(pathItem, 0755); err != nil {
				return paths, err
			}
		}
	}
	return paths, nil
}

func byteCountToString(size int64) string {
	if size < 1 {
		return "? bytes"
	}
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d bytes", size)
	}
	div, exp := int64(unit), 0
	for i := size / unit; i >= unit; i /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

func (this *PackageInstaller) revertFileSystem(paths []string) (err error) {
	for _, path := range paths {
		err = errors.Join(err, ignoreNotExist(this.filesystem.Delete(path)))
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

var decompressors = map[string]func(source io.Reader, showProgress bool) (io.ReadCloser, error){
	"zstd": newZStdReader,
	"zip":  newZipReader,
	"gzip": newGZipReader,
}

func newZStdReader(source io.Reader, _ bool) (io.ReadCloser, error) {
	if reader, err := zstd.NewReader(source); err != nil {
		return nil, err
	} else {
		return reader.IOReadCloser(), nil
	}
}
func newZipReader(source io.Reader, showProgress bool) (io.ReadCloser, error) {
	return shell.NewZipArchiveReader(source, showProgress)
}
func newGZipReader(source io.Reader, _ bool) (io.ReadCloser, error) {
	return gzip.NewReader(source)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type ArchiveReader interface {
	Next() (*tar.Header, error)
	io.Reader
}

var archiveFormats = map[string]func(reader io.Reader) ArchiveReader{
	"": func(reader io.Reader) ArchiveReader { return tar.NewReader(reader) },
}

func closeResource(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}
