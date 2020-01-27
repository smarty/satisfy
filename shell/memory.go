package shell

import (
	"bytes"
	"io"
	"io/ioutil"
	"sort"
	"time"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type InMemoryFileSystem struct {
	fileSystem map[string]*file
	Root       string
}

func NewInMemoryFileSystem() *InMemoryFileSystem {
	return &InMemoryFileSystem{
		fileSystem: make(map[string]*file),
	}
}

func (this *InMemoryFileSystem) Stat(path string) contracts.FileInfo {
	return this.fileSystem[path]
}

func (this *InMemoryFileSystem) Listing() (files []contracts.FileInfo) {
	for _, file := range this.fileSystem {
		files = append(files, file)
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path() < files[j].Path() })
	return files
}

func (this *InMemoryFileSystem) Open(path string) io.ReadCloser {
	target := this.fileSystem[path]
	if target.symlink != "" {
		target = this.fileSystem[target.symlink]
	}
	return ioutil.NopCloser(bytes.NewReader(target.contents))
}

func (this *InMemoryFileSystem) Create(path string) io.WriteCloser {
	this.WriteFile(path, nil)
	return this.fileSystem[path]
}

func (this *InMemoryFileSystem) ReadFile(path string) []byte {
	target := this.fileSystem[path]
	if target.symlink != "" {
		target = this.fileSystem[target.symlink]
	}
	return target.contents
}

func (this *InMemoryFileSystem) WriteFile(path string, content []byte) {
	this.fileSystem[path] = &file{
		path:     path,
		contents: content,
		mod:      InMemoryModTime,
	}
}

func (this *InMemoryFileSystem) WriteSymLink(target, source string) {
	this.fileSystem[target] = &file{
		path:     target,
		contents: nil,
		mod:      InMemoryModTime,
		symlink:  source,
	}
}

func (this *InMemoryFileSystem) Delete(path string) {
	this.fileSystem[path] = nil
	delete(this.fileSystem, path)
}

func (this *InMemoryFileSystem) RootPath() string {
	return this.Root
}

/////////////////////////////////////////////////

type file struct {
	path     string
	contents []byte
	mod      time.Time
	symlink  string
}

func (this *file) Symlink() string { return this.symlink }

var InMemoryModTime = time.Now()

func (this *file) ModTime() time.Time {
	return this.mod
}

func (this *file) Write(p []byte) (n int, err error) {
	this.contents = append(this.contents, p...)
	return len(p), nil
}

func (this *file) Close() error {
	return nil
}

func (this *file) Path() string {
	return this.path
}

func (this *file) Size() int64 {
	return int64(len(this.contents))
}
