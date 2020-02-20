package contracts

import "time"

type ArchiveWriter interface {
	Write([]byte) (int, error)
	Close() error
	WriteHeader(ArchiveHeader)
}

type ArchiveHeader struct {
	Name     string
	Size     int64
	ModTime  time.Time
	LinkName string
	Executable bool
}
