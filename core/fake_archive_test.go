package core

import (
	"errors"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type ArchiveItem struct {
	contracts.ArchiveHeader
	contents []byte
}

type FakeArchiveWriter struct {
	items       []*ArchiveItem
	current     *ArchiveItem
	closed      bool
	writeError  error
	closedError error
}

func NewFakeArchiveWriter() *FakeArchiveWriter { return &FakeArchiveWriter{} }
func (this *FakeArchiveWriter) WriteHeader(header contracts.ArchiveHeader) {
	if this.closed {
		return
	}
	this.current = &ArchiveItem{ArchiveHeader: header}
	this.items = append(this.items, this.current)
}
func (this *FakeArchiveWriter) Write(p []byte) (int, error) {
	this.current.contents = append(this.current.contents, p...)
	return len(p), this.writeError

}
func (this *FakeArchiveWriter) Close() error {
	this.closed = true
	return this.closedError
}

var (
	writeErr = errors.New("write error")
	closeErr = errors.New("close error")
)
