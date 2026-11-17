package archive

import (
	"io"
	"math"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/smarty/satisfy/v2/internal/model"
)

var (
	suffixes = [5]string{"B", "KB", "MB", "GB", "TB"}
)

func round(val float64, roundOn float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}

// HumanFileSize renders a byte count using binary (1024) units, e.g. "1.5 MB".
func HumanFileSize(size int64) string {
	return humanFileSize(float64(size))
}

func humanFileSize(size float64) string {
	if size < 1 {
		return "0 B"
	}
	base := math.Log(size) / math.Log(1024)
	getSize := round(math.Pow(1024, base-math.Floor(base)), .5, 2)
	getSuffix := suffixes[int(math.Floor(base))]
	return strconv.FormatFloat(getSize, 'f', -1, 64) + " " + getSuffix
}

type archiveProgressCounter struct {
	written    atomic.Int64
	total      int64
	onProgress func(written, total int64, done bool)
	printTimer *time.Ticker
	done       chan struct{}
	stopped    chan struct{}
}

func (this *archiveProgressCounter) Write(p []byte) (n int, e error) {
	n = len(p)
	this.written.Add(int64(n))
	return
}

func (this *archiveProgressCounter) Read(p []byte) (n int, err error) {
	n = len(p)
	this.written.Add(int64(n))
	return
}

// Close stops the periodic reports, then reports once more with done set, so
// that the final report is always the last.
func (this *archiveProgressCounter) Close() error {
	this.printTimer.Stop()
	close(this.done)
	<-this.stopped
	this.reportProgress(true)
	return nil
}

func (this *archiveProgressCounter) reportProgress(done bool) {
	this.onProgress(this.written.Load(), this.total, done)
}

// NewProgressCounter counts the bytes written to it and reports them through
// progress, for the item name in the given phase, every two seconds and once
// more (with Done) when closed. With a nil progress it does nothing, without
// starting a goroutine.
func NewProgressCounter(progress model.ProgressFunc, phase model.ProgressPhase, name string, size int64) io.WriteCloser {
	if progress == nil {
		return discardCounter{}
	}
	return NewArchiveProgressCounter(size, func(written, total int64, done bool) {
		progress(model.Progress{Phase: phase, Name: name, Written: written, Total: total, Done: done})
	})
}

type discardCounter struct{}

func (discardCounter) Write(p []byte) (int, error) { return len(p), nil }
func (discardCounter) Close() error                { return nil }

// NewArchiveProgressCounter counts the bytes written to it and calls
// onProgress every two seconds, and once more (with done) when closed.
func NewArchiveProgressCounter(size int64, onProgress func(written, total int64, done bool)) io.WriteCloser {
	return newArchiveProgressCounter(size, 2*time.Second, onProgress)
}

func newArchiveProgressCounter(size int64, interval time.Duration, onProgress func(written, total int64, done bool)) *archiveProgressCounter {
	this := &archiveProgressCounter{total: size, onProgress: onProgress}
	this.printTimer = time.NewTicker(interval)
	this.done = make(chan struct{})
	this.stopped = make(chan struct{})
	go func() {
		defer close(this.stopped)
		for {
			select {
			case <-this.printTimer.C:
				this.reportProgress(false)
			case <-this.done:
				return
			}
		}
	}()
	return this
}
