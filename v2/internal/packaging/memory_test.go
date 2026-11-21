package packaging

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/smarty/satisfy/v2/internal/model"
)

type inMemoryFileSystem struct {
	fileSystem   map[string]*file
	Root         string
	errReadFile  map[string]error
	errChmodFile map[string]error
	errOpen      map[string]error
	errCreate    map[string]error
	errListing   error
}

func newInMemoryFileSystem() *inMemoryFileSystem {
	return &inMemoryFileSystem{
		fileSystem:   make(map[string]*file),
		errReadFile:  make(map[string]error),
		errChmodFile: make(map[string]error),
		errOpen:      make(map[string]error),
		errCreate:    make(map[string]error),
	}
}

func (this *inMemoryFileSystem) Chmod(name string, mode os.FileMode) error {
	this.fileSystem[name].mode = mode
	return this.errChmodFile[name]
}

func (this *inMemoryFileSystem) Stat(path string) (model.FileInfo, error) {
	file, found := this.fileSystem[path]
	if found {
		return file, nil
	} else {
		return file, os.ErrNotExist
	}
}

func (this *inMemoryFileSystem) Listing() (files []model.FileInfo, err error) {
	for _, file := range this.fileSystem {
		files = append(files, file)
	}

	slices.SortFunc(files, func(a, b model.FileInfo) int { return strings.Compare(a.Path(), b.Path()) })
	return files, this.errListing
}

func (this *inMemoryFileSystem) listing() []model.FileInfo {
	files, _ := this.Listing()
	return files
}

func (this *inMemoryFileSystem) Open(path string) (io.ReadCloser, error) {
	if err := this.errOpen[path]; err != nil {
		return nil, err
	}
	file, found := this.fileSystem[path]
	if !found {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(file.contents)), nil
}

func (this *inMemoryFileSystem) Create(path string) (io.WriteCloser, error) {
	if err := this.errCreate[path]; err != nil {
		return nil, err
	}
	_ = this.WriteFile(path, nil)
	return this.fileSystem[path], nil
}

func (this *inMemoryFileSystem) readFile(path string) []byte {
	target := this.fileSystem[path]
	if target.symlink != "" {
		target = this.resolveSymlink(target)
	}
	return target.contents
}

func (this *inMemoryFileSystem) ReadFile(path string) ([]byte, error) {
	_, found := this.fileSystem[path]
	if !found {
		return nil, os.ErrNotExist
	}
	return this.readFile(path), this.errReadFile[path]
}

func (this *inMemoryFileSystem) resolveSymlink(target *file) *file {
	source, found := this.fileSystem[target.symlink]
	if found {
		return source
	}
	parts := strings.Split(target.path, string(os.PathSeparator))
	for part := 1; part < len(parts); part++ {
		prepend := filepath.Join(parts[:part]...)
		path := filepath.Join(prepend, target.symlink)
		source, found := this.fileSystem[path]
		if found {
			return source
		}
	}
	return nil
}

func (this *inMemoryFileSystem) WriteFile(path string, content []byte) error {
	this.fileSystem[path] = &file{
		path:     path,
		contents: content,
		mod:      InMemoryModTime,
		mode:     os.FileMode(0),
	}
	return nil
}

func (this *inMemoryFileSystem) WriteDirectory(path string) {
	this.fileSystem[path] = &file{
		path: path,
		mod:  InMemoryModTime,
		mode: fs.ModeDir,
	}
}

func (this *inMemoryFileSystem) CreateSymlink(source, target string) error {
	this.fileSystem[target] = &file{
		path:     target,
		contents: nil,
		mod:      InMemoryModTime,
		symlink:  source,
	}
	return nil
}

func (this *inMemoryFileSystem) Delete(path string) error {
	if _, found := this.fileSystem[path]; !found {
		return os.ErrNotExist
	}
	delete(this.fileSystem, path)
	return nil
}

func (this *inMemoryFileSystem) OpenRoot(dir string) (model.RootedFileSystem, error) {
	return inMemoryRoot{fileSystem: this, dir: dir}, nil
}

func (this *inMemoryFileSystem) RootPath() string {
	return this.Root
}

/////////////////////////////////////////////////

// inMemoryRoot confines operations to dir, refusing names that leave it, as an
// os.Root does.
type inMemoryRoot struct {
	fileSystem *inMemoryFileSystem
	dir        string
}

func (this inMemoryRoot) path(name string) (string, error) {
	if !filepath.IsLocal(name) {
		return "", fmt.Errorf("path escapes from parent: %q", name)
	}
	return filepath.Join(this.dir, name), nil
}
func (this inMemoryRoot) Create(name string) (io.WriteCloser, error) {
	path, err := this.path(name)
	if err != nil {
		return nil, err
	}
	return this.fileSystem.Create(path)
}
func (this inMemoryRoot) CreateSymlink(source, target string) error {
	path, err := this.path(target)
	if err != nil {
		return err
	}
	return this.fileSystem.CreateSymlink(source, path)
}
func (this inMemoryRoot) Chmod(name string, mode os.FileMode) error {
	path, err := this.path(name)
	if err != nil {
		return err
	}
	return this.fileSystem.Chmod(path, mode)
}
func (this inMemoryRoot) Delete(name string) error {
	path, err := this.path(name)
	if err != nil {
		return err
	}
	return this.fileSystem.Delete(path)
}
func (this inMemoryRoot) Close() error { return nil }

/////////////////////////////////////////////////

type file struct {
	path     string
	contents []byte
	mod      time.Time
	symlink  string
	mode     os.FileMode
}

func (this *file) Write(p []byte) (n int, err error) {
	this.contents = append(this.contents, p...)
	return len(p), nil
}

var InMemoryModTime = time.Now()

func (this *file) ModTime() time.Time { return this.mod }
func (this *file) Symlink() string    { return this.symlink }
func (this *file) Close() error       { return nil }
func (this *file) Path() string       { return this.path }
func (this *file) Size() int64        { return int64(len(this.contents)) }
func (this *file) Mode() os.FileMode  { return this.mode }
