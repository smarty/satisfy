package main

import "io"

func NopReadSeekCloser(r io.ReadSeeker) ReadSeekCloser {
	return nopReadSeekCloser{r}
}

type ReadSeekCloser interface {
	io.ReadSeeker
	io.Closer
}

type nopReadSeekCloser struct {
	io.ReadSeeker
}

func (nopReadSeekCloser) Close() error {
	return nil
}
