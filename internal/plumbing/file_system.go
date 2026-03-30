package plumbing

import (
	"io"
	"os"
	"time"
)

type PathLister interface {
	Listing() ([]FileInfo, error)
}

type FileOpener interface {
	Open(path string) (io.ReadCloser, error)
}

type FileCreator interface {
	Create(path string) (io.WriteCloser, error)
}

type SymlinkCreator interface {
	CreateSymlink(source, target string) error
}

type FileReader interface {
	ReadFile(path string) ([]byte, error)
}

type FileWriter interface {
	WriteFile(path string, content []byte) error
}

type Deleter interface {
	Delete(path string) error
}

type FileChecker interface {
	Stat(path string) (FileInfo, error)
}

type FileInfo interface {
	Path() string
	Size() int64
	ModTime() time.Time
	Symlink() string
	Mode() os.FileMode
}

type RootPath interface {
	RootPath() string
}

type Chmod interface {
	Chmod(name string, mode os.FileMode) error
}
