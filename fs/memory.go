package fs

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"time"
)

type file struct {
	*bytes.Buffer
}

func (this *file) Name() string {
	panic("implement me")
}

func (this *file) Size() int64 {
	panic("implement me")
}

func (this *file) Mode() os.FileMode {
	panic("implement me")
}

func (this *file) ModTime() time.Time {
	panic("implement me")
}

func (this *file) IsDir() bool {
	panic("implement me")
}

func (this *file) Sys() interface{} {
	panic("implement me")
}

type InMemoryFileSystem struct {
	fileSystem map[string]*file
}

func NewInMemoryFileSystem() *InMemoryFileSystem {
	return &InMemoryFileSystem{fileSystem:make(map[string]*file)}
}

func (this *InMemoryFileSystem) Listing(root string) []os.FileInfo {
	panic("implement me")
}

func (this *InMemoryFileSystem) Open(path string) io.ReadCloser {
	return ioutil.NopCloser(this.fileSystem[path])
}

func (this *InMemoryFileSystem) Create(path string) io.WriteCloser {
	panic("implement me")
}

func (this *InMemoryFileSystem) ReadFile(path string) []byte {
	return this.fileSystem[path].Bytes()
}

func (this *InMemoryFileSystem) WriteFile(path string, content []byte) {
	this.fileSystem[path] = &file{Buffer: bytes.NewBuffer(content)}
}

