package shell

import "io"

type nopWriteCloser struct{}

func (nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (nopWriteCloser) Close() error                { return nil }

func noopProgress(_ int64) io.WriteCloser { return nopWriteCloser{} }
