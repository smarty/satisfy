package core

import (
	"io"
	"math"
	"strconv"
	"time"
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
	written    int64
	total      string
	onProgress func(written string, total string)
	printTimer *time.Ticker
	done       chan struct{}
}

func (this *archiveProgressCounter) Write(p []byte) (n int, e error) {
	n = len(p)
	this.written += int64(n)
	return
}

func (this *archiveProgressCounter) Close() error {
	this.reportProgress()
	this.printTimer.Stop()
	close(this.done)
	return nil
}

func (this *archiveProgressCounter) reportProgress() {
	this.onProgress(humanFileSize(float64(this.written)), this.total)
}

func newArchiveProgressCounter(size int64, onProgress func(written, total string)) io.WriteCloser {
	this := &archiveProgressCounter{total: humanFileSize(float64(size)), onProgress: onProgress}
	this.printTimer = time.NewTicker(2 * time.Second)
	this.done = make(chan struct{})
	go func() {
		for {
			select {
			case <-this.printTimer.C:
				this.reportProgress()
			case <-this.done:
				return
			}
		}
	}()
	return this
}
