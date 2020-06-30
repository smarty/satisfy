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
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/smartystreets/logging"
	"github.com/smartystreets/satisfy/contracts"
)

type PackageInstallerFileSystem interface {
	contracts.FileCreator
	contracts.FileWriter
	contracts.Deleter
	contracts.SymlinkCreator
	contracts.Chmod
}

type PackageInstaller struct {
	logger     *logging.Logger
	downloader contracts.Downloader
	filesystem PackageInstallerFileSystem
}

func NewPackageInstaller(downloader contracts.Downloader, filesystem PackageInstallerFileSystem) *PackageInstaller {
	return &PackageInstaller{downloader: downloader, filesystem: filesystem}
}

func (this *PackageInstaller) DownloadManifest(remoteAddress url.URL) (manifest contracts.Manifest, err error) {
	body, err := this.downloader.Download(remoteAddress)
	if err != nil {
		return contracts.Manifest{}, err
	}

	defer func() { _ = body.Close() }()

	rawManifest, err := ioutil.ReadAll(body)
	err = json.Unmarshal(rawManifest, &manifest)

	return manifest, err
}

func (this *PackageInstaller) InstallManifest(request contracts.InstallationRequest) (manifest contracts.Manifest, err error) {
	manifest, err = this.DownloadManifest(request.RemoteAddress)
	if err != nil {
		return contracts.Manifest{}, err
	}
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

	defer func() { _ = body.Close() }()
	hashReader := NewHashReader(body, md5.New())

	factory, found := decompressors[manifest.Archive.CompressionAlgorithm]
	if !found {
		return errors.New("invalid compression algorithm")
	}
	decompressor, err := factory(hashReader)
	if err != nil {
		return err
	}
	paths, err := this.extractArchive(decompressor, request, len(manifest.Archive.Contents))
	if err != nil {
		this.revertFileSystem(paths)
		return err
	}
	actualChecksum := hashReader.Sum(nil)
	if bytes.Compare(actualChecksum, manifest.Archive.MD5Checksum) != 0 {
		this.revertFileSystem(paths)
		return fmt.Errorf("checksum mismatch: %x != %x", actualChecksum, manifest.Archive.MD5Checksum)
	}

	return nil
}

func (this *PackageInstaller) extractArchive(decompressor io.Reader, request contracts.InstallationRequest, itemCount int) (paths []string, err error) {
	var reader ArchiveReader
	if archiveReader, ok := decompressor.(ArchiveReader); ok {
		reader = archiveReader
	} else {
		reader = archiveFormats[""](decompressor)
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
		this.logger.Printf("Extracting archive item [%d/%d] \"%s\" [%s] to \"%s\".",
			i+1, itemCount, header.Name, byteCountToString(header.Size), pathItem)

		if header.Typeflag == tar.TypeSymlink {
			this.filesystem.CreateSymlink(header.Linkname, pathItem)
		} else {
			writer := this.filesystem.Create(pathItem)
			_, err = io.Copy(writer, reader)
			_ = writer.Close()
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

var decompressors = map[string]func(_ io.Reader) (io.Reader, error){
	"zstd": func(reader io.Reader) (io.Reader, error) { return zstd.NewReader(reader) },
	"gzip": func(reader io.Reader) (io.Reader, error) { return gzip.NewReader(reader) },
}

type ArchiveReader interface {
	Next() (*tar.Header, error)
	io.Reader
}

var archiveFormats = map[string]func(reader io.Reader) ArchiveReader{
	"": func(reader io.Reader) ArchiveReader { return tar.NewReader(reader) },
}
