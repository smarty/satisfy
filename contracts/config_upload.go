package contracts

import (
	"net/url"
	"path"

	"github.com/smarty/gcs"
)

type UploadConfig struct {
	MaxRetry          int
	GoogleCredentials gcs.Credentials
	JSONPath          string
	Overwrite         bool
	PackageConfig     PackageConfig
}

type PackageConfig struct {
	CompressionAlgorithm string `json:"compression_algorithm"`
	CompressionLevel     int    `json:"compression_level"`
	SourceDirectory      string `json:"source_directory"`
	PackageName          string `json:"package_name"`
	PackageVersion       string `json:"package_version"`
	RemoteAddressPrefix  *URL   `json:"remote_address"`
}

func (this PackageConfig) ComposeRemoteAddress(filename string) url.URL {
	return AppendRemotePath(url.URL(*this.RemoteAddressPrefix), this.PackageName, this.PackageVersion, filename)
}

func (this PackageConfig) ComposeLatestManifestRemoteAddress() url.URL {
	address := url.URL(*this.RemoteAddressPrefix)
	address.Path = path.Join(address.Path, this.PackageName, RemoteManifestFilename)
	return address
}
