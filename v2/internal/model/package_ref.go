package model

import (
	"net/url"
	"strings"
)

// PackageRef identifies a package on remote storage: the address under which
// packages live and the package's name.
type PackageRef struct {
	RemoteAddress URL    `json:"remote_address"`
	Name          string `json:"package_name"`
}

// Normalized returns the reference with leading and trailing slashes trimmed
// from the name.
func (this PackageRef) Normalized() PackageRef {
	this.Name = strings.Trim(this.Name, "/")
	return this
}

// ComposeManifestAddress returns the address of the version's manifest, or of
// the root manifest when version is "latest".
func (this PackageRef) ComposeManifestAddress(version string) url.URL {
	return AppendRemotePath(url.URL(this.RemoteAddress), this.Name, version, RemoteManifestFilename)
}

// ComposeRootManifestAddress returns the address of the root manifest, which
// records the latest version and the package's tags.
func (this PackageRef) ComposeRootManifestAddress() url.URL {
	return this.ComposeManifestAddress("latest")
}
