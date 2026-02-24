package contracts

import (
	"net/url"
	"path"
	"strings"
)

const (
	RemoteArchiveFilename  = "archive"
	RemoteManifestFilename = "manifest.json"
)

type URL url.URL

func (this *URL) MarshalJSON() ([]byte, error) {
	return []byte(`"` + this.Value().String() + `"`), nil
}

func (this *URL) UnmarshalJSON(p []byte) error {
	raw := string(p)
	if raw == `"null"` {
		return nil
	}

	raw = strings.Trim(raw, "\"")
	address, err := url.Parse(raw)
	if err == nil {
		*this = URL(*address)
	}

	return err
}

func (this URL) Value() *url.URL {
	standard := url.URL(this)
	return &standard
}

func AppendRemotePath(prefix url.URL, packageName, version, fileName string) url.URL {
	if version == "latest" {
		prefix.Path = path.Join(prefix.Path, packageName, fileName)
	} else {
		prefix.Path = path.Join(prefix.Path, packageName, version, fileName)
	}

	if !strings.HasPrefix(prefix.Path, "/") {
		prefix.Path = "/" + prefix.Path
	}

	return prefix
}
