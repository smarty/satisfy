package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/internal/plumbing"
	"github.com/smarty/satisfy/internal/shell"
)

// Compile-time check of interface implementations.
var _ plumbing.DownloadSetter = (*shell.ZipArchiveReader)(nil)

type PackageInstallerFileSystem interface {
	plumbing.FileCreator
	plumbing.FileWriter
	plumbing.Deleter
	plumbing.SymlinkCreator
	plumbing.Chmod
}

type PackageInstaller struct {
	downloader  plumbing.Downloader
	filesystem  PackageInstallerFileSystem
	emit        func(contracts.Event)
	newProgress func(int64) io.WriteCloser
}

func NewPackageInstaller(downloader plumbing.Downloader, filesystem PackageInstallerFileSystem, newProgress func(int64) io.WriteCloser, emit func(contracts.Event)) *PackageInstaller {
	if newProgress == nil {
		newProgress = noopProgress
	}

	if emit == nil {
		emit = func(contracts.Event) {}
	}

	return &PackageInstaller{downloader: downloader, filesystem: filesystem, emit: emit, newProgress: newProgress}
}

func (this *PackageInstaller) DownloadManifest(remoteAddress url.URL) (manifest plumbing.Manifest, err error) {
	body, err := this.downloader.Download(remoteAddress)
	if err != nil {
		return plumbing.Manifest{}, err
	}

	defer closeResource(body)

	rawManifest, err := io.ReadAll(body)
	err = json.Unmarshal(rawManifest, &manifest)

	return manifest, err
}

func (this *PackageInstaller) InstallManifest(request plumbing.InstallationRequest) (manifest plumbing.Manifest, err error) {
	manifest, err = this.DownloadManifest(request.RemoteAddress)
	if err != nil {
		return plumbing.Manifest{}, err
	}

	manifest.Name = request.PackageName
	rawManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return plumbing.Manifest{}, err
	}

	if err = this.filesystem.WriteFile(ComposeManifestPath(request.LocalPath, manifest.Name), rawManifest); err != nil {
		return plumbing.Manifest{}, err
	}
	return manifest, nil
}

func (this *PackageInstaller) InstallPackage(manifest plumbing.Manifest, request plumbing.InstallationRequest) error {
	body, err := this.downloader.Download(request.RemoteAddress)
	if err != nil {
		return err
	}

	defer closeResource(body)
	checksumReader := NewHashReader(body, md5.New())

	factory, found := this.decompressor(manifest.Archive.CompressionAlgorithm)
	if !found {
		return errors.New("invalid compression algorithm")
	}
	decompressor, err := factory(checksumReader)
	if err != nil {
		return err
	}
	paths, err := this.extractArchive(decompressor, request, len(manifest.Archive.Contents))
	if err != nil {
		this.revertFileSystem(paths)
		return err
	}
	actualChecksum := checksumReader.Sum(nil)
	if bytes.Compare(actualChecksum, manifest.Archive.MD5Checksum) != 0 {
		this.revertFileSystem(paths)
		return fmt.Errorf("checksum mismatch: actual [%x] != expected [%x]", actualChecksum, manifest.Archive.MD5Checksum)
	}

	return nil
}

func (this *PackageInstaller) extractArchive(decompressor io.ReadCloser, request plumbing.InstallationRequest, itemCount int) (paths []string, err error) {
	defer closeResource(decompressor)
	var reader ArchiveReader
	if archiveReader, ok := decompressor.(ArchiveReader); ok {
		reader = archiveReader
	} else {
		reader = archiveFormats[""](decompressor)
	}

	if _, ok := reader.(plumbing.DownloadSetter); ok {
		reader.(plumbing.DownloadSetter).SetDownloader(request.RemoteAddress, this.downloader)
	}

	for i := 0; ; i++ {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return paths, err
		}
		pathItem := filepath.Join(request.LocalPath, header.Name)
		paths = append(paths, pathItem)
		this.emit(contracts.Event{
			Type:    contracts.EventProgress,
			Message: fmt.Sprintf("Extracting archive item [%d/%d] %q [%s] to %q.", i+1, itemCount, header.Name, byteCountToString(header.Size), pathItem),
		})

		if header.Typeflag == tar.TypeSymlink {
			if err = this.filesystem.CreateSymlink(header.Linkname, pathItem); err != nil {
				return paths, err
			}
		} else {
			writer, err := this.filesystem.Create(pathItem)
			if err != nil {
				return paths, err
			}
			progressReader := this.newProgress(header.Size)
			multiWriter := io.MultiWriter(writer, progressReader)
			_, err = io.Copy(multiWriter, reader)
			_ = writer.Close()
			_ = progressReader.Close()
			if err != nil {
				return paths, err
			}
			if !IsExecutable(os.FileMode(header.Mode)) {
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

func (this *PackageInstaller) revertFileSystem(paths []string) {
	for _, path := range paths {
		_ = this.filesystem.Delete(path)
	}
}

func ComposeManifestPath(localPath, packageName string) string {
	cleanPackageName := strings.ReplaceAll(packageName, "/", "___")
	fileName := fmt.Sprintf("manifest_%s.json", cleanPackageName)
	return filepath.Join(localPath, fileName)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (this *PackageInstaller) decompressor(algorithm string) (func(io.Reader) (io.ReadCloser, error), bool) {
	if algorithm == "zip" {
		return func(r io.Reader) (io.ReadCloser, error) {
			reader, err := shell.NewZipArchiveReader(r, this.newProgress)
			if err != nil {
				return nil, err
			}
			return reader, nil
		}, true
	}

	factory, found := decompressors[algorithm]
	return factory, found
}

var decompressors = map[string]func(_ io.Reader) (io.ReadCloser, error){
	"gzip": newGZipReader,
	"zstd": newZStdReader,
}

func newZStdReader(source io.Reader) (io.ReadCloser, error) {
	if reader, err := zstd.NewReader(source); err != nil {
		return nil, err
	} else {
		return reader.IOReadCloser(), nil
	}
}
func newGZipReader(source io.Reader) (io.ReadCloser, error) {
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

// //////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
type DownloadSetter interface {
	SetDownloader(url.URL, plumbing.Downloader)
}

func closeResource(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}

func IsExecutable(mode os.FileMode) bool {
	return mode.Perm()&0111 > 0
}
