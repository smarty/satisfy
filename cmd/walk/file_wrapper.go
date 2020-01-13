package main

import "io"

type FileWrapper struct {
	io.ReadSeeker
}

func NewFileWrapper(inner io.ReadSeeker) *FileWrapper {
	return &FileWrapper{ReadSeeker: inner}
}

func (this *FileWrapper) Close() error {
	return nil
}
