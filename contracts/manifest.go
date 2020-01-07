package contracts

import "time"

type Manifest struct {
	Name     string     `json:"name"`
	Version  string     `json:"version"`
	Created  time.Time  `json:"created"`
	Contents []FileInfo `json:"contents"`
}

type FileInfo struct {
	Path        string `json:"path"`
	Size        int    `json:"size"`
	MD5Checksum []byte `json:"md5_checksum"`
	Permissions int    `json:"permissions"`
}
