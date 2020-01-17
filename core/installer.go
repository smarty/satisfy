package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

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
	body, err := this.downloader.Download(request.DownloadRequest)
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
	body, err := this.downloader.Download(request.DownloadRequest)
	if err != nil {
		return err
	}
	hashReader := NewHashReader(body, md5.New())
	gzipReader, err := gzip.NewReader(hashReader)
	if err != nil {
		return err
	}
	err = this.extractArchive(gzipReader, request)
	if err != nil {
		return err
	}
	actualChecksum := hashReader.Sum(nil)
	if bytes.Compare(actualChecksum, manifest.Archive.MD5Checksum) != 0 {
		// TODO: Clean up fileSystem
		return fmt.Errorf("checksum mistmatch: %x != %x", actualChecksum, manifest.Archive.MD5Checksum)
	}

	return nil
}

func (this *PackageInstaller) extractArchive(gzipReader *gzip.Reader, request contracts.InstallationRequest) error {
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		path := filepath.Join(request.LocalPath, header.Name)
		writer := this.filesystem.Create(path)
		_, err = io.Copy(writer, tarReader)
		if err != nil {
			return err
		}
	}
	return nil
}

func composeManifestPath(localPath string,manifest contracts.Manifest) string {
	cleanPackageName := strings.ReplaceAll(manifest.Name, "/", "|")
	fileName := fmt.Sprintf("manifest_%s_%s.json", cleanPackageName, manifest.Version)
	return filepath.Join(localPath, fileName)
}
