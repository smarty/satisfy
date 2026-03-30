package plumbing

import "time"

type ArchiveWriter interface {
	Write([]byte) (int, error)
	Close() error
	WriteHeader(ArchiveHeader) error
}

type ArchiveHeader struct {
	Name       string
	Size       int64
	ModTime    time.Time
	LinkName   string
	Executable bool
}
