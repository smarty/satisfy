package contracts

import (
	"io"
	"time"
)

type FileSystem interface {
	Listing() []FileInfo
	Open(path string) io.ReadCloser
	Create(path string) io.WriteCloser
	ReadFile(path string) []byte
	WriteFile(path string, content []byte)
}

type FileInfo interface {
	Path() string
	Size() int64
	ModTime() time.Time
}
