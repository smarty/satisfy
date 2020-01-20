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

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type PackageInstaller struct {
	downloader contracts.Downloader
	filesystem contracts.FileSystem
}

func NewPackageInstaller(downloader contracts.Downloader, filesystem contracts.FileSystem) *PackageInstaller {
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
	file := this.filesystem.Create(composeManifestPath(localPath, manifest))
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
		return fmt.Errorf("checksum mistmatch: %x != %x", actualChecksum, manifest.Archive.MD5Checksum)
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
		writer := this.filesystem.Create(path)
		_, err = io.Copy(writer, tarReader)
		if err != nil {
			return paths, err
		}
	}
	return paths, nil
}

func (this *PackageInstaller) revertFileSystem(paths []string) {
	for _, path := range paths {
		this.filesystem.Delete(path)
	}
}

func composeManifestPath(localPath string, manifest contracts.Manifest) string {
	cleanPackageName := strings.ReplaceAll(manifest.Name, "/", "|")
	fileName := fmt.Sprintf("manifest_%s_%s.json", cleanPackageName, manifest.Version)
	return filepath.Join(localPath, fileName)
}

var decompression = map[string]func(_ io.Reader) (io.Reader, error){
	"zstd": func(reader io.Reader) (io.Reader, error) {
		decompressor, err := zstd.NewReader(reader)
		if err != nil {
			return nil, err
		}
		return decompressor, nil
	},
	"gzip": func(reader io.Reader) (io.Reader, error) {
		decompressor, err := gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
		return decompressor, nil
	},
}
