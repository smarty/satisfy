package contracts

import (
	"io"
)

type ArchiveWriter interface {
	io.WriteCloser
	WriteHeader(name string, size, mode int64)
}
