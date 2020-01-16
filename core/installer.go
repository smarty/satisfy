package core

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type PackageInstaller struct {
	downloader contracts.Downloader
	filesystem contracts.FileSystem
}

func (this *PackageInstaller) InstallManifest(request contracts.InstallationRequest) (manifest contracts.Manifest, err error) {
	readcloser, err := this.downloader.Download(request.DownloadRequest)
	if err != nil {
		return manifest, err
	}
	decoder := json.NewDecoder(readcloser)
	err = decoder.Decode(&manifest)
	if err != nil {
		return manifest, err
	}
	file := this.filesystem.Create(filepath.Join(request.LocalPath, fmt.Sprintf("manifest_%s_%s.json", strings.ReplaceAll(manifest.Name, "/", "|"), manifest.Version)))
	encoder := json.NewEncoder(file)
	_ = encoder.Encode(manifest)

	return manifest, nil
}

func NewPackageInstaller(downloader contracts.Downloader, filesystem contracts.FileSystem) *PackageInstaller {
	return &PackageInstaller{downloader: downloader, filesystem: filesystem}
}
