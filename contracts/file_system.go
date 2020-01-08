package contracts

import "io"

type FileSystem interface {
	Listing() []FileInfo        //filepath.Walk
	Open(path string) io.ReadCloser        //os.Open
	Create(path string) io.WriteCloser     //os.Create
	ReadFile(path string) []byte           //ioutil.ReadFile
	WriteFile(path string, content []byte) //ioutil.WriteFile
}

type FileInfo interface {
	Path() string
	Size() int64
}
