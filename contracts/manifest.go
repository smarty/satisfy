package contracts

type Manifest struct {
	Name    string  `json:"name"` //a-z 0-9 _-/
	Version string  `json:"version"`
	Archive Archive `json:"archive"`
}

type Archive struct {
	Filename    string        `json:"filename"`
	Size        uint64        `json:"size"`
	MD5Checksum []byte        `json:"md5"`
	Contents    []ArchiveItem `json:"contents"`
}

type ArchiveItem struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	MD5Checksum []byte `json:"md5"`
}
