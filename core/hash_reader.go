package core

import (
	"hash"
	"io"
)

type HashReader struct {
	io.Reader
	hash.Hash
}

func NewHashReader(source io.Reader, target hash.Hash) *HashReader {
	return &HashReader{Reader: source, Hash: target}
}

func (this *HashReader) Read(buffer []byte) (int, error) {
	count, err := this.Reader.Read(buffer)
	_, _ = this.Hash.Write(buffer[0:count])
	return count, err
}
