package contracts

type Manifest struct {
	Name    string  `json:"name"` //a-z 0-9 _-/
	Version string  `json:"version"`
	Archive Archive `json:"archive"`
	Tags    []Tag   `json:"tags,omitempty"` // only present in the root (latest) manifest
}

type Tag struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (this Manifest) TagVersion(name string) (version string, found bool) {
	for _, tag := range this.Tags {
		if tag.Name == name {
			return tag.Version, true
		}
	}
	return "", false
}

type Archive struct {
	Filename             string        `json:"filename"`
	Size                 uint64        `json:"size"`
	MD5Checksum          []byte        `json:"md5"`
	Contents             []ArchiveItem `json:"contents"`
	CompressionAlgorithm string        `json:"compression"`
}

type ArchiveItem struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	MD5Checksum []byte `json:"md5"`
}
