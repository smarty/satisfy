package contracts

import (
	"io"
	"time"
)

type ArchiveWriter interface {
	Write([]byte) (int, error)
	Close() error
	WriteHeader(ArchiveHeader)
}

type ArchiveReader interface {
	Next() bool
	Header() ArchiveHeader
	Reader() io.Reader
}

type ArchiveHeader struct {
	Name    string
	Size    int64
	ModTime time.Time
}
