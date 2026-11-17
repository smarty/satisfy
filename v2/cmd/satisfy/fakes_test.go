package main

import (
	"io"
	"log/slog"
	"os"
)

// inMemoryFileSystem is the subset of a file system the config loaders read.
type inMemoryFileSystem struct {
	files map[string][]byte
}

func newInMemoryFileSystem() *inMemoryFileSystem {
	return &inMemoryFileSystem{files: make(map[string][]byte)}
}

func (this *inMemoryFileSystem) ReadFile(path string) ([]byte, error) {
	content, found := this.files[path]
	if !found {
		return nil, os.ErrNotExist
	}
	return content, nil
}

func (this *inMemoryFileSystem) WriteFile(path string, content []byte) {
	this.files[path] = content
}

func (this *inMemoryFileSystem) Delete(path string) {
	delete(this.files, path)
}

func newTestLogger(output io.Writer) *slog.Logger {
	return newLogger(output)
}
