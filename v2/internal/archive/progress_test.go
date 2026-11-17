package archive

import (
	"testing"
	"time"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestArchiveProgressCounter(t *testing.T) {
	gunit.Run(new(ArchiveProgressCounter), t)
}

type ArchiveProgressCounter struct {
	*gunit.Fixture
	written int64
	total   int64
}

func (this *ArchiveProgressCounter) Setup() {
}

func (this *ArchiveProgressCounter) TestHumanFileSizeWithZero() {
	fileProgress := humanFileSize(0)
	this.So(fileProgress, should.Equal, "0 B")
}

func (this *ArchiveProgressCounter) TestRound() {
	rounded := round(26.2248, .5, 3)
	this.So(rounded, should.Equal, 26.225)
}

func (this *ArchiveProgressCounter) LongTestProgress() {
	a := NewArchiveProgressCounter(250_000_000, func(written, total int64, done bool) {
		this.written = written
		this.total = total
	})
	a.Write([]byte{'t', 'e', 's', 't'})
	time.Sleep(3 * time.Second)
	this.So(this.written, should.Equal, 4)
	a.Write([]byte{'t', 'e', 's', 't'})
	a.Close()
	this.So(this.written, should.Equal, 8)
	this.So(HumanFileSize(this.total), should.Equal, "238.42 MB")
}
