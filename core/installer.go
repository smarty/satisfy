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
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/smartystreets/logging"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type PackageInstallerFileSystem interface {
	contracts.FileCreator
	contracts.Deleter
	contracts.SymlinkCreator
}

type PackageInstaller struct {
	logger     *logging.Logger
	downloader contracts.Downloader
	filesystem PackageInstallerFileSystem
}

func NewPackageInstaller(downloader contracts.Downloader, filesystem PackageInstallerFileSystem) *PackageInstaller {
	return &PackageInstaller{downloader: downloader, filesystem: filesystem}
}

func (this *PackageInstaller) InstallManifest(request contracts.InstallationRequest) (manifest contracts.Manifest, err error) {
	body, err := this.downloader.Download(request.RemoteAddress)
	if err != nil {
		return contracts.Manifest{}, err
	}

	err = json.NewDecoder(body).Decode(&manifest)
	if err != nil {
		return contracts.Manifest{}, err
	}

	this.writeLocalManifest(request.LocalPath, manifest)
	return manifest, nil
}

func (this *PackageInstaller) writeLocalManifest(localPath string, manifest contracts.Manifest) {
	// TODO: any particular reason to re-serialize the manifest?
	file := this.filesystem.Create(ComposeManifestPath(localPath, manifest.Name))
	defer func() { _ = file.Close() }()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "   ")
	_ = encoder.Encode(manifest)
}

func (this *PackageInstaller) InstallPackage(manifest contracts.Manifest, request contracts.InstallationRequest) error {
	body, err := this.downloader.Download(request.RemoteAddress)
	if err != nil {
		return err
	}
	hashReader := NewHashReader(body, md5.New())

	factory, found := decompressors[manifest.Archive.CompressionAlgorithm]
	if !found {
		return errors.New("invalid compression algorithm")
	}
	decompressor, err := factory(hashReader)
	if err != nil {
		return err
	}
	paths, err := this.extractArchive(decompressor, request)
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

func (this *PackageInstaller) extractArchive(decompressor io.Reader, request contracts.InstallationRequest) (paths []string, err error) {
	tarReader := tar.NewReader(decompressor)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return paths, err
		}
		pathItem := filepath.Join(request.LocalPath, header.Name)
		paths = append(paths, pathItem)
		this.logger.Printf("Extracting archive item \"%s\" to \"%s\".", header.Name, pathItem)

		if header.Typeflag == tar.TypeSymlink {
			this.filesystem.CreateSymlink(header.Linkname, pathItem)
		} else {
			writer := this.filesystem.Create(pathItem)
			_, err = io.Copy(writer, tarReader)
			_ = writer.Close()
			if err != nil {
				return paths, err
			}
		}
	}
	return paths, nil
}

func (this *PackageInstaller) revertFileSystem(paths []string) {
	for _, path := range paths {
		this.filesystem.Delete(path)
	}
}

func ComposeManifestPath(localPath, name string) string {
	cleanPackageName := strings.ReplaceAll(name, "/", "|")
	fileName := fmt.Sprintf("manifest_%s.json", cleanPackageName)
	return filepath.Join(localPath, fileName)
}

var decompressors = map[string]func(_ io.Reader) (io.Reader, error){
	"zstd": func(reader io.Reader) (io.Reader, error) { return zstd.NewReader(reader) },
	"gzip": func(reader io.Reader) (io.Reader, error) { return gzip.NewReader(reader) },
}
