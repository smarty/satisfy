package contracts

import (
	"net/url"
	"path"
)

type PackageConfig struct {
	CompressionAlgorithm string `json:"compression_algorithm"`
	CompressionLevel     int    `json:"compression_level"`
	SourceDirectory      string `json:"source_directory"`
	SourceFile           string `json:"source_file"`
	SourcePath           string `json:"source_path"`
	PackageName          string `json:"package_name"`
	PackageVersion       string `json:"package_version"`
	RemoteAddressPrefix  *URL   `json:"remote_address"`
}

func (this PackageConfig) ComposeLatestManifestRemoteAddress() url.URL {
	address := url.URL(*this.RemoteAddressPrefix)
	address.Path = path.Join(address.Path, this.PackageName, RemoteManifestFilename)
	return address
}

func (this PackageConfig) ComposeRemoteAddress(filename string) url.URL {
	return AppendRemotePath(url.URL(*this.RemoteAddressPrefix), this.PackageName, this.PackageVersion, filename)
}
