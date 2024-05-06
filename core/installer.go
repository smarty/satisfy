package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/klauspost/compress/zstd"
	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/shell"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Compile-time check of interface implementations.
var _ contracts.DownloadSetter = (*shell.ZipArchiveReader)(nil)

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

func (this *PackageInstaller) DownloadManifest(remoteAddress url.URL) (manifest contracts.Manifest, err error) {
	body, err := this.downloader.Download(remoteAddress)
	if err != nil {
		return contracts.Manifest{}, err
	}

	defer closeResource(body)

	rawManifest, err := io.ReadAll(body)
	err = json.Unmarshal(rawManifest, &manifest)

	return manifest, err
}

func (this *PackageInstaller) InstallManifest(request contracts.InstallationRequest) (manifest contracts.Manifest, err error) {
	manifest, err = this.DownloadManifest(request.RemoteAddress)
	if err != nil {
		return contracts.Manifest{}, err
	}

	manifest.Name = request.PackageName
	rawManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return contracts.Manifest{}, err
	}
	this.filesystem.WriteFile(ComposeManifestPath(request.LocalPath, manifest.Name), rawManifest)
	return manifest, nil
}

func (this *PackageInstaller) InstallPackage(manifest contracts.Manifest, request contracts.InstallationRequest) error {
	body, err := this.downloader.Download(request.RemoteAddress)
	if err != nil {
		return err
	}

	defer closeResource(body)
	checksumReader := NewHashReader(body, md5.New())

	factory, found := decompressors[manifest.Archive.CompressionAlgorithm]
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

func (this *PackageInstaller) extractArchive(decompressor io.ReadCloser, request contracts.InstallationRequest, itemCount int) (paths []string, err error) {
	defer closeResource(decompressor)
	var reader ArchiveReader
	if archiveReader, ok := decompressor.(ArchiveReader); ok {
		reader = archiveReader
	} else {
		reader = archiveFormats[""](decompressor)
	}

	if _, ok := reader.(contracts.DownloadSetter); ok {
		reader.(contracts.DownloadSetter).SetDownloader(request.RemoteAddress, this.downloader)
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
			progressReader := newArchiveProgressCounter(header.Size, func(archived, total string, done bool) {
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
			_ = writer.Close()
			_ = progressReader.Close()
			if err != nil {
				return paths, err
			}
			if !contracts.IsExecutable(os.FileMode(header.Mode)) {
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

var decompressors = map[string]func(_ io.Reader) (io.ReadCloser, error){
	"zstd": newZStdReader,
	"zip":  newZipReader,
	"gzip": newGZipReader,
}

func newZStdReader(source io.Reader) (io.ReadCloser, error) {
	if reader, err := zstd.NewReader(source); err != nil {
		return nil, err
	} else {
		return reader.IOReadCloser(), nil
	}
}
func newZipReader(source io.Reader) (io.ReadCloser, error) {
	return shell.NewZipArchiveReader(source).(io.ReadCloser), nil
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
	SetDownloader(url.URL, contracts.Downloader)
}

func closeResource(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}
