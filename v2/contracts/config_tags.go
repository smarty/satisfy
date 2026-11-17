package contracts

import (
	"net/url"
	"path"

	"github.com/smarty/gcs"
)

type TagsConfig struct {
	MaxRetry          int
	GoogleCredentials gcs.Credentials
	JSONPath          string
	Modification      TagModificationConfig
}

type TagModificationConfig struct {
	PackageName   string `json:"package_name"`
	RemoteAddress *URL   `json:"remote_address"`
	Add           []Tag  `json:"add"`
	Delete        []Tag  `json:"delete"` // only the name of each entry is considered
}

func (this TagModificationConfig) ComposeRootManifestAddress() url.URL {
	address := url.URL(*this.RemoteAddress)
	address.Path = path.Join("/", address.Path, this.PackageName, RemoteManifestFilename)
	return address
}

func (this TagModificationConfig) ComposeVersionedManifestAddress(version string) url.URL {
	return AppendRemotePath(url.URL(*this.RemoteAddress), this.PackageName, version, RemoteManifestFilename)
}
