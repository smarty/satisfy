package core

import (
	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"testing"
	"time"
)

func TestArchiveProgressCounter(t *testing.T) {
	gunit.Run(new(ArchiveProgressCounter), t)
}

type ArchiveProgressCounter struct {
	*gunit.Fixture
	written string
	total   string
}

func (this *ArchiveProgressCounter) Setup() {
}

func (this *ArchiveProgressCounter) TestHumanFileSizeWithZero() {
	fileProgress := humanFileSize(0)
	this.So(fileProgress, should.Equal, "0 B")
}

func (this *ArchiveProgressCounter) TestRound() {
	rounded := round(26.2245, .5, 3)
	this.So(rounded, should.Equal, 26.225)
}

func (this *ArchiveProgressCounter) LongTestProgress() {
	a := newArchiveProgressCounter(250_000_000.00, func(written, total string, done bool) {
		this.written = written
		this.total = total
	})
	a.Write([]byte{'t', 'e', 's', 't'})
	time.Sleep(3 * time.Second)
	this.So(this.written, should.Equal, "4 B")
	a.Write([]byte{'t', 'e', 's', 't'})
	a.Close()
	this.So(this.written, should.Equal, "8 B")
	this.So(this.total, should.Equal, "238.42 MB")
}
