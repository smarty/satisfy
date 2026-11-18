package fsys

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/smarty/satisfy/v2/internal/model"
)

type DiskFileSystem struct{ root string }

func NewDiskFileSystem(root string) *DiskFileSystem {
	return &DiskFileSystem{root: filepath.Clean(root)}
}

func (this *DiskFileSystem) RootPath() string {
	return this.root
}

func (this *DiskFileSystem) Listing() (listing []model.FileInfo, err error) {
	listingFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		fileInfo := FileInfo{
			path: path,
			size: info.Size(),
			mod:  info.ModTime(),
			mode: info.Mode(),
		}
		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			fileInfo.symlink, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		listing = append(listing, fileInfo)
		return nil
	}
	stat, err := os.Stat(this.root)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		err = listingFunc(this.root, stat, nil)
	} else {
		err = filepath.Walk(this.root, listingFunc)
	}
	if err != nil {
		return nil, err
	}
	return listing, nil
}

func (this *DiskFileSystem) Stat(path string) (model.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	fileInfo := FileInfo{
		path: path,
		size: info.Size(),
		mod:  info.ModTime(),
		mode: info.Mode(),
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if fileInfo.symlink, err = os.Readlink(path); err != nil {
			return nil, err
		}
	}
	return fileInfo, nil
}

// OpenRoot creates dir if needed and returns a file system confined to it by
// an os.Root: no name, symlink or ".." can reach outside dir.
func (this *DiskFileSystem) OpenRoot(dir string) (model.RootedFileSystem, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return diskRoot{root: root}, nil
}

func (this *DiskFileSystem) Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (this *DiskFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (this *DiskFileSystem) WriteFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

////////////////////////////////////////

type diskRoot struct{ root *os.Root }

func (this diskRoot) Create(name string) (io.WriteCloser, error) {
	if err := this.root.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return nil, err
	}
	return this.root.Create(name)
}

func (this diskRoot) CreateSymlink(source, target string) error {
	if err := this.root.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	if err := this.root.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return this.root.Symlink(source, target)
}

func (this diskRoot) Chmod(name string, mode os.FileMode) error {
	return this.root.Chmod(name, mode)
}

func (this diskRoot) Delete(name string) error {
	return this.root.Remove(name)
}

func (this diskRoot) Close() error {
	return this.root.Close()
}

////////////////////////////////////////

type FileInfo struct {
	path    string
	size    int64
	mod     time.Time
	symlink string
	mode    os.FileMode
}

func (this FileInfo) Path() string       { return this.path }
func (this FileInfo) Size() int64        { return this.size }
func (this FileInfo) ModTime() time.Time { return this.mod }
func (this FileInfo) Symlink() string    { return this.symlink }
func (this FileInfo) Mode() os.FileMode  { return this.mode }
