package core

import (
	"hash"
	"io"
)

type HashReader struct {
	io.Reader
	hash.Hash
}

func NewHashReader(inner io.Reader, hash hash.Hash) *HashReader {
	return &HashReader{Reader: inner, Hash: hash}
}

func (this *HashReader) Read(p []byte) (n int, err error) {
	n, err = this.Reader.Read(p)
	this.Hash.Write(p[:n])
	return n, err
}

