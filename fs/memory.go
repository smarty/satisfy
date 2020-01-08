package fs

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type file struct {
	name string
	*bytes.Buffer
}

func (this *file) Close() error {
	return nil
}

func (this *file) Name() string {
	return this.name
}

func (this *file) Size() int64 {
	return int64(this.Len())
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
	return &InMemoryFileSystem{fileSystem: make(map[string]*file)}
}

func (this *InMemoryFileSystem) Listing(root string) (files []os.FileInfo) {
	for path, file := range this.fileSystem {
		if strings.Contains(path, root) {
			files = append(files, file)
		}
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
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
	this.fileSystem[path] = &file{name: filepath.Base(path), Buffer: bytes.NewBuffer(content)}
}
