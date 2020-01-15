package core

import (
	"encoding/json"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type PackageInstaller struct {
	downloader contracts.Downloader
	filesystem contracts.FileSystem
}

func (this *PackageInstaller) InstallManifest(request contracts.InstallationRequest) (contracts.Manifest, error) {
	readcloser, _ := this.downloader.Download(request.DownloadRequest)
	decoder := json.NewDecoder(readcloser)
	var manifest contracts.Manifest
	decoder.Decode(&manifest)
	return manifest, nil
}

func NewPackageInstaller(downloader contracts.Downloader, filesystem contracts.FileSystem) *PackageInstaller {
	return &PackageInstaller{downloader: downloader, filesystem: filesystem}
}

