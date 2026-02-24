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
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/smarty/satisfy/internal/shell"
	"github.com/smarty/satisfy/legacy_contracts"
)

// Compile-time check of interface implementations.
var _ legacy_contracts.DownloadSetter = (*shell.ZipArchiveReader)(nil)

type PackageInstallerFileSystem interface {
	legacy_contracts.FileCreator
	legacy_contracts.FileWriter
	legacy_contracts.Deleter
	legacy_contracts.SymlinkCreator
	legacy_contracts.Chmod
}

type PackageInstaller struct {
	downloader  legacy_contracts.Downloader
	filesystem  PackageInstallerFileSystem
	newProgress func(int64) io.WriteCloser
}

func NewPackageInstaller(downloader legacy_contracts.Downloader, filesystem PackageInstallerFileSystem, newProgress func(int64) io.WriteCloser) *PackageInstaller {
	if newProgress == nil {
		newProgress = noopProgress
	}

	return &PackageInstaller{downloader: downloader, filesystem: filesystem, newProgress: newProgress}
}

func (this *PackageInstaller) DownloadManifest(remoteAddress url.URL) (manifest legacy_contracts.Manifest, err error) {
	body, err := this.downloader.Download(remoteAddress)
	if err != nil {
		return legacy_contracts.Manifest{}, err
	}

	defer closeResource(body)

	rawManifest, err := io.ReadAll(body)
	err = json.Unmarshal(rawManifest, &manifest)

	return manifest, err
}

func (this *PackageInstaller) InstallManifest(request legacy_contracts.InstallationRequest) (manifest legacy_contracts.Manifest, err error) {
	manifest, err = this.DownloadManifest(request.RemoteAddress)
	if err != nil {
		return legacy_contracts.Manifest{}, err
	}

	manifest.Name = request.PackageName
	rawManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return legacy_contracts.Manifest{}, err
	}

	this.filesystem.WriteFile(ComposeManifestPath(request.LocalPath, manifest.Name), rawManifest)
	return manifest, nil
}

func (this *PackageInstaller) InstallPackage(manifest legacy_contracts.Manifest, request legacy_contracts.InstallationRequest) error {
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

func (this *PackageInstaller) extractArchive(decompressor io.ReadCloser, request legacy_contracts.InstallationRequest, itemCount int) (paths []string, err error) {
	defer closeResource(decompressor)
	var reader ArchiveReader
	if archiveReader, ok := decompressor.(ArchiveReader); ok {
		reader = archiveReader
	} else {
		reader = archiveFormats[""](decompressor)
	}

	if _, ok := reader.(legacy_contracts.DownloadSetter); ok {
		reader.(legacy_contracts.DownloadSetter).SetDownloader(request.RemoteAddress, this.downloader)
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
		log.Printf("Extracting archive item [%d/%d] \"%s\" [%s] to \"%s\".",
			i+1, itemCount, header.Name, byteCountToString(header.Size), pathItem)

		if header.Typeflag == tar.TypeSymlink {
			this.filesystem.CreateSymlink(header.Linkname, pathItem)
		} else {
			writer := this.filesystem.Create(pathItem)
			progressReader := this.newProgress(header.Size)
			multiWriter := io.MultiWriter(writer, progressReader)
			_, err = io.Copy(multiWriter, reader)
			_ = writer.Close()
			_ = progressReader.Close()
			if err != nil {
				return paths, err
			}
			if !legacy_contracts.IsExecutable(os.FileMode(header.Mode)) {
				continue
			}
			err := this.filesystem.Chmod(pathItem, 0755)
			if err != nil {
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
		this.filesystem.Delete(path)
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
			return shell.NewZipArchiveReader(r, this.newProgress), nil
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
	SetDownloader(url.URL, legacy_contracts.Downloader)
}

func closeResource(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}
