package archive

import (
	"sync"
	"testing"
	"time"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2/internal/model"
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

// The counter is written on the caller's goroutine while the ticker goroutine
// reads it; run with -race.
func (this *ArchiveProgressCounter) TestConcurrentWritesAndReportsAreSafeAndDoneIsLast() {
	var reports []bool // done flag of each report, in order
	var lock sync.Mutex
	counter := newArchiveProgressCounter(1000, time.Microsecond, func(written, total int64, done bool) {
		lock.Lock()
		defer lock.Unlock()
		reports = append(reports, done)
	})
	for range 1000 {
		_, _ = counter.Write([]byte{'x'})
		time.Sleep(time.Microsecond)
	}
	this.So(counter.Close(), should.BeNil)

	lock.Lock()
	defer lock.Unlock()
	this.So(len(reports), should.BeGreaterThan, 1)
	this.So(reports[len(reports)-1], should.BeTrue)
	this.So(counter.written.Load(), should.Equal, 1000)
}

func (this *ArchiveProgressCounter) TestWithoutProgressFuncNothingIsCounted() {
	counter := NewProgressCounter(nil, model.ProgressArchiving, "item", 10)
	this.So(counter, should.Resemble, discardCounter{})
	n, err := counter.Write([]byte("abc"))
	this.So(n, should.Equal, 3)
	this.So(err, should.BeNil)
	this.So(counter.Close(), should.BeNil)
}

func (this *ArchiveProgressCounter) TestProgressFuncReceivesItemReports() {
	var reports []model.Progress
	counter := NewProgressCounter(func(progress model.Progress) { reports = append(reports, progress) },
		model.ProgressExtracting, "item", 10)
	_, _ = counter.Write([]byte("abc"))
	this.So(counter.Close(), should.BeNil)
	this.So(reports, should.Resemble, []model.Progress{
		{Phase: model.ProgressExtracting, Name: "item", Written: 3, Total: 10, Done: true},
	})
}
