package model

import (
	"net/url"
	"path"
)

type PackageConfig struct {
	CompressionAlgorithm string   `json:"compression_algorithm"`
	CompressionLevel     int      `json:"compression_level"` // 0 => algorithm's default level
	SourceDirectory      string   `json:"source_directory"`
	SourceFile           string   `json:"source_file"`
	SourcePath           string   `json:"source_path"`
	PackageName          string   `json:"package_name"`
	PackageVersion       string   `json:"package_version"`
	RemoteAddress        URL      `json:"remote_address"`
	Tags                 []string `json:"tags,omitempty"`
}

func (this PackageConfig) ComposeRemoteAddress(filename string) url.URL {
	return AppendRemotePath(url.URL(this.RemoteAddress), this.PackageName, this.PackageVersion, filename)
}

func (this PackageConfig) ComposeLatestManifestRemoteAddress() url.URL {
	address := url.URL(this.RemoteAddress)
	address.Path = path.Join(address.Path, this.PackageName, RemoteManifestFilename)
	return address
}

// Ref returns the reference to the package being uploaded.
func (this PackageConfig) Ref() PackageRef {
	return PackageRef{RemoteAddress: this.RemoteAddress, Name: this.PackageName}
}
