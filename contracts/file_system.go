package contracts

import (
	"io"
	"os"
)

type FileSystem interface {
	Listing(root string) []os.FileInfo  //filepath.Walk
	Open(path string) io.ReadCloser		//os.Open
	Create(path string) io.WriteCloser	//os.Create
	ReadFile(path string) []byte		//ioutil.ReadFile
	WriteFile(path string, content []byte)	//ioutil.WriteFile
}
