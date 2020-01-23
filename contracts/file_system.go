package contracts

import (
	"io"
	"time"
)

// FUTURE: make each file system path return any underlying error.

type PathLister interface {
	Listing() []FileInfo
}

type FileOpener interface {
	Open(path string) io.ReadCloser
}

type FileCreator interface {
	Create(path string) io.WriteCloser
}

type FileReader interface {
	ReadFile(path string) []byte
}

type FileWriter interface {
	WriteFile(path string, content []byte)
}

type Deleter interface {
	Delete(path string)
}

type FileInfo interface {
	Path() string
	Size() int64
	ModTime() time.Time
}
