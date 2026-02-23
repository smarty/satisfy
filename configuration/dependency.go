package configuration

import (
	"fmt"
	"net/url"
	"path"
)

type Dependency struct {
	PackageName    string `json:"package_name"`
	PackageVersion string `json:"package_version"`
	RemoteAddress  URL    `json:"remote_address"`
	LocalDirectory string `json:"local_directory"`
}

func (this Dependency) ComposeLatestManifestRemoteAddress() url.URL {
	address := url.URL(this.RemoteAddress)
	address.Path = "/" + path.Join(address.Path, this.PackageName, RemoteManifestFilename)
	return address
}

func (this Dependency) ComposeRemoteAddress(fileName string) url.URL {
	return AppendRemotePath(
		url.URL(this.RemoteAddress),
		this.PackageName,
		this.PackageVersion,
		fileName,
	)
}

func (this Dependency) ComposeRemoteManifestAddress() url.URL {
	if this.PackageVersion == "latest" {
		return this.ComposeLatestManifestRemoteAddress()
	} else {
		return this.ComposeRemoteAddress(RemoteManifestFilename)
	}
}

func (this Dependency) Title() string {
	return fmt.Sprintf("[%s @ %s]", this.PackageName, this.PackageVersion)
}
