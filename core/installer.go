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
}

type PackageInstaller struct {
	logger *logging.Logger
	downloader contracts.Downloader
	filesystem PackageInstallerFileSystem
}

func NewPackageInstaller(downloader contracts.Downloader, filesystem PackageInstallerFileSystem) *PackageInstaller {
	return &PackageInstaller{downloader: downloader, filesystem: filesystem}
}

func (this *PackageInstaller) InstallManifest(request contracts.InstallationRequest) (manifest contracts.Manifest, err error) {
	body, err := this.downloader.Download(request.RemoteAddress)
	if err != nil {
		return manifest, err
	}

	err = json.NewDecoder(body).Decode(&manifest)
	if err != nil {
		return manifest, err
	}

	this.writeLocalManifest(request.LocalPath, manifest)

	return manifest, nil
}

func (this *PackageInstaller) writeLocalManifest(localPath string, manifest contracts.Manifest) {
	file := this.filesystem.Create(ComposeManifestPath(localPath, manifest.Name))
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "   ")
	_ = encoder.Encode(manifest)
	_ = file.Close()
}

func (this *PackageInstaller) InstallPackage(manifest contracts.Manifest, request contracts.InstallationRequest) error {
	body, err := this.downloader.Download(request.RemoteAddress)
	if err != nil {
		return err
	}
	hashReader := NewHashReader(body, md5.New())

	factory, found := decompression[manifest.Archive.CompressionAlgorithm]
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
		path := filepath.Join(request.LocalPath, header.Name)
		paths = append(paths, path)
		this.logger.Printf("Extracting archive item \"%s\" to \"%s\".", header.Name, path)
		writer := this.filesystem.Create(path)
		_, err = io.Copy(writer, tarReader)
		if err != nil {
			return paths, err
		}
		_ = writer.Close()
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

var decompression = map[string]func(_ io.Reader) (io.Reader, error){
	"zstd": func(reader io.Reader) (io.Reader, error) { return zstd.NewReader(reader) },
	"gzip": func(reader io.Reader) (io.Reader, error) { return gzip.NewReader(reader) },
}
