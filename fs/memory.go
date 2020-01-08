package fs

import (
	"bytes"
	"io"
	"sort"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type file struct {
	path string
	*bytes.Buffer
}

func (this *file) Close() error {
	return nil
}

func (this *file) Path() string {
	return this.path
}

func (this *file) Size() int64 {
	return int64(this.Len())
}

type InMemoryFileSystem struct {
	fileSystem map[string]*file
}

func NewInMemoryFileSystem() *InMemoryFileSystem {
	return &InMemoryFileSystem{fileSystem: make(map[string]*file)}
}

func (this *InMemoryFileSystem) Listing() (files []contracts.FileInfo) {
	for _, file := range this.fileSystem {
		files = append(files, file)
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path() < files[j].Path() })
	return files
}

func (this *InMemoryFileSystem) Open(path string) io.ReadCloser {
	return this.fileSystem[path]
}

func (this *InMemoryFileSystem) Create(path string) io.WriteCloser {
	this.WriteFile(path, nil)
	return this.fileSystem[path]
}

func (this *InMemoryFileSystem) ReadFile(path string) []byte {
	return this.fileSystem[path].Bytes()
}

func (this *InMemoryFileSystem) WriteFile(path string, content []byte) {
	this.fileSystem[path] = &file{path: path, Buffer: bytes.NewBuffer(content)}
}
