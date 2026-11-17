package satisfy

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestRunConcurrentlyFixture(t *testing.T) {
	gunit.Run(new(RunConcurrentlyFixture), t)
}

type RunConcurrentlyFixture struct {
	*gunit.Fixture
}

func (this *RunConcurrentlyFixture) TestEveryItemIsProcessed() {
	var processed atomic.Int32
	errs := runConcurrently(context.Background(), 2, []int{1, 2, 3, 4, 5}, func(int) error {
		processed.Add(1)
		return nil
	})
	this.So(errs, should.BeEmpty)
	this.So(processed.Load(), should.Equal, 5)
}

func (this *RunConcurrentlyFixture) TestNoMoreThanLimitRunAtOnce() {
	var running, peak atomic.Int32
	_ = runConcurrently(context.Background(), 3, make([]int, 20), func(int) error {
		now := running.Add(1)
		for {
			observed := peak.Load()
			if now <= observed || peak.CompareAndSwap(observed, now) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		running.Add(-1)
		return nil
	})
	this.So(peak.Load(), should.BeBetweenOrEqual, 1, 3)
}

func (this *RunConcurrentlyFixture) TestErrorsAreCollected() {
	failure := errors.New("failure")
	errs := runConcurrently(context.Background(), 4, []int{1, 2, 3, 4}, func(item int) error {
		if item%2 == 0 {
			return failure
		}
		return nil
	})
	this.So(errs, should.HaveLength, 2)
	this.So(errs[0], should.Equal, failure)
}

func (this *RunConcurrentlyFixture) TestLimitBelowOneStillRuns() {
	errs := runConcurrently(context.Background(), 0, []int{1}, func(int) error { return nil })
	this.So(errs, should.BeEmpty)
}

func (this *RunConcurrentlyFixture) TestNoWorkStartsOnceTheContextIsDone() {
	ctx, cancel := context.WithCancel(context.Background())
	var started atomic.Int32
	release := make(chan struct{})

	errs := runConcurrently(ctx, 1, make([]int, 10), func(int) error {
		if started.Add(1) == 1 {
			cancel() // while the only slot is taken, so the next item waits
			close(release)
		}
		<-release
		return nil
	})

	this.So(started.Load(), should.Equal, 1)
	this.So(errs, should.Resemble, []error{context.Canceled})
}

func (this *RunConcurrentlyFixture) TestAlreadyCancelledContextStartsNothing() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var started atomic.Int32

	errs := runConcurrently(ctx, 4, make([]int, 10), func(int) error {
		started.Add(1)
		return nil
	})

	this.So(started.Load(), should.Equal, 0)
	this.So(errs, should.Resemble, []error{context.Canceled})
}
