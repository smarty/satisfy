package model

import (
	"net/url"
)

type TagModificationConfig struct {
	PackageName   string `json:"package_name"`
	RemoteAddress URL    `json:"remote_address"`
	Add           []Tag  `json:"add"`
	Delete        []Tag  `json:"delete"` // only the name of each entry is considered
}

func (this TagModificationConfig) ComposeRootManifestAddress() url.URL {
	return AppendRemotePath(url.URL(this.RemoteAddress), this.PackageName, "latest", RemoteManifestFilename)
}

func (this TagModificationConfig) ComposeVersionedManifestAddress(version string) url.URL {
	return AppendRemotePath(url.URL(this.RemoteAddress), this.PackageName, version, RemoteManifestFilename)
}

// Ref returns the reference to the package whose tags are modified.
func (this TagModificationConfig) Ref() PackageRef {
	return PackageRef{RemoteAddress: this.RemoteAddress, Name: this.PackageName}
}
