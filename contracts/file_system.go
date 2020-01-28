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

type SymlinkCreator interface {
	CreateSymlink(source, target string)
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

type FileChecker interface {
	Stat(path string) FileInfo
}

type FileInfo interface {
	Path() string
	Size() int64
	ModTime() time.Time
	Symlink() string
}

type RootPath interface {
	RootPath() string
}
