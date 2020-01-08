package contracts

type Manifest struct {
	Name    string `json:"short_name"` //a-z 0-9 _-/
	Version string `json:"version"`
	Archive Archive
}

type Archive struct { // TODO add json tags
	Filename    string
	Size        uint64
	MD5Checksum []byte
	Contents    []ArchiveItem
}

type ArchiveItem struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	MD5Checksum []byte `json:"md5_checksum"`
}
