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

func (this *DiskFileSystem) CreateDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}

func NewDiskFileSystem(root string) *DiskFileSystem {
	return &DiskFileSystem{root: root}
}

func (this *DiskFileSystem) Listing() (listing []contracts.FileInfo) {
	err := filepath.Walk(this.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(this.root, path)
		if err != nil {
			return err
		}
		listing = append(listing, FileInfo{
			path: relative,
			size: info.Size(),
			mod:  info.ModTime(),
		})
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return listing
}

func (this *DiskFileSystem) Open(path string) io.ReadCloser {
	reader, err := os.Open(this.absolute(path))
	if err != nil {
		log.Panic(err)
	}
	return reader
}

func (this *DiskFileSystem) Create(path string) io.WriteCloser {
	writer, err := os.Create(this.absolute(path))
	if err != nil {
		log.Panic(err)
	}
	return writer
}

func (this *DiskFileSystem) ReadFile(path string) []byte {
	raw, err := ioutil.ReadFile(this.absolute(path))
	if err != nil {
		log.Panic(err)
	}
	return raw
}

func (this *DiskFileSystem) WriteFile(path string, content []byte) {
	err := ioutil.WriteFile(this.absolute(path), content, 0644)
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

func (this *DiskFileSystem) absolute(path string) string {
	return filepath.Join(this.root, path)
}

////////////////////////////////////////

type FileInfo struct {
	path string
	size int64
	mod  time.Time
}

func (this FileInfo) Path() string       { return this.path }
func (this FileInfo) Size() int64        { return this.size }
func (this FileInfo) ModTime() time.Time { return this.mod }
