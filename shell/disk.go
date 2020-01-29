package shell

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type DiskFileSystem struct{ root string }

func NewDiskFileSystem(root string) *DiskFileSystem {
	return &DiskFileSystem{root: filepath.Clean(root)}
}

func (this *DiskFileSystem) RootPath() string {
	return this.root
}

func (this *DiskFileSystem) Listing() (listing []contracts.FileInfo) {
	err := filepath.Walk(this.root, func(path string, info os.FileInfo, err error) error {
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
		}
		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			fileInfo.symlink, err = os.Readlink(path) // TODO: support for absolute AND relative symlinks
			if err != nil {
				return err
			}
		}
		listing = append(listing, fileInfo)
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return listing
}

func (this *DiskFileSystem) Stat(path string) (contracts.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	fileInfo := FileInfo{
		path: path,
		size: info.Size(),
		mod:  info.ModTime(),
	}
	return fileInfo, nil
}

func (this *DiskFileSystem) CreateSymlink(source, target string) {
	_ = os.Remove(target)
	err := os.Symlink(source, target)
	if err != nil {
		log.Panic(err)
	}
}

func (this *DiskFileSystem) Open(path string) io.ReadCloser {
	reader, err := os.Open(path)
	if err != nil {
		log.Panic(err)
	}
	return reader
}

func (this *DiskFileSystem) Create(path string) io.WriteCloser {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		log.Panic(err)
	}
	writer, err := os.Create(path)
	if err != nil {
		log.Panic(err)
	}
	return writer
}

func (this *DiskFileSystem) ReadFile(path string) []byte {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panic(err)
	}
	return raw
}

func (this *DiskFileSystem) WriteFile(path string, content []byte) {
	err := ioutil.WriteFile(path, content, 0644)
	if err != nil {
		log.Panic(err)
	}
}

func (this *DiskFileSystem) Delete(path string) {
	err := os.Remove(path)
	if err != nil {
		log.Panic(err)
	}
}

////////////////////////////////////////

type FileInfo struct {
	path    string
	size    int64
	mod     time.Time
	symlink string
}

func (this FileInfo) Path() string       { return this.path }
func (this FileInfo) Size() int64        { return this.size }
func (this FileInfo) ModTime() time.Time { return this.mod }
func (this FileInfo) Symlink() string    { return this.symlink }
