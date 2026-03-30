package shell

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/smarty/satisfy/internal/plumbing"
)

type DiskFileSystem struct{ root string }

func NewDiskFileSystem(root string) *DiskFileSystem {
	return &DiskFileSystem{root: filepath.Clean(root)}
}

func (this *DiskFileSystem) Chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}

func (this *DiskFileSystem) RootPath() string {
	return this.root
}

func (this *DiskFileSystem) Listing() (listing []plumbing.FileInfo, err error) {
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
		if err != nil {
			return nil, err
		}
	} else {
		err = filepath.Walk(this.root, listingFunc)
		if err != nil {
			return nil, err
		}
	}
	return listing, nil
}

func (this *DiskFileSystem) Stat(path string) (plumbing.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	source, _ := os.Readlink(path)
	fileInfo := FileInfo{
		path:    path,
		size:    info.Size(),
		mod:     info.ModTime(),
		symlink: source,
		mode:    info.Mode(),
	}
	return fileInfo, nil
}

func (this *DiskFileSystem) CreateSymlink(source, target string) error {
	_ = os.Remove(target)
	return os.Symlink(source, target)
}

func (this *DiskFileSystem) Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (this *DiskFileSystem) Create(path string) (io.WriteCloser, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	return os.Create(path)
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

func (this *DiskFileSystem) Delete(path string) error {
	return os.Remove(path)
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
