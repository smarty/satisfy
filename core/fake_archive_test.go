package core

import (
	"bytes"
	"errors"
	"io"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type ArchiveItem struct {
	contracts.ArchiveHeader
	contents []byte
}

type FakeArchive struct {
	i           int
	items       []*ArchiveItem
	current     *ArchiveItem
	closed      bool
	writeError  error
	closedError error
}

func NewFakeArchive() *FakeArchive { return &FakeArchive{i: -1} }

func (this *FakeArchive) Next() bool {
	this.i++
	return this.i < len(this.items)
}

func (this *FakeArchive) Header() contracts.ArchiveHeader {
	return this.items[this.i].ArchiveHeader
}

func (this *FakeArchive) Reader() io.Reader {
	return bytes.NewReader(this.items[this.i].contents)
}

func (this *FakeArchive) WriteHeader(header contracts.ArchiveHeader) {
	if this.closed {
		return
	}
	this.current = &ArchiveItem{ArchiveHeader: header}
	this.items = append(this.items, this.current)
}
func (this *FakeArchive) Write(p []byte) (int, error) {
	this.current.contents = append(this.current.contents, p...)
	return len(p), this.writeError

}
func (this *FakeArchive) Close() error {
	this.closed = true
	return this.closedError
}

var (
	writeErr = errors.New("write error")
	closeErr = errors.New("close error")
)
