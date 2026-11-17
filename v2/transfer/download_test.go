package transfer

import (
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
	errs := runConcurrently(2, []int{1, 2, 3, 4, 5}, func(int) error {
		processed.Add(1)
		return nil
	})
	this.So(errs, should.BeEmpty)
	this.So(processed.Load(), should.Equal, 5)
}

func (this *RunConcurrentlyFixture) TestNoMoreThanLimitRunAtOnce() {
	var running, peak atomic.Int32
	_ = runConcurrently(3, make([]int, 20), func(int) error {
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
	errs := runConcurrently(4, []int{1, 2, 3, 4}, func(item int) error {
		if item%2 == 0 {
			return failure
		}
		return nil
	})
	this.So(errs, should.HaveLength, 2)
	this.So(errs[0], should.Equal, failure)
}

func (this *RunConcurrentlyFixture) TestLimitBelowOneStillRuns() {
	errs := runConcurrently(0, []int{1}, func(int) error { return nil })
	this.So(errs, should.BeEmpty)
}

func TestAccessTokenRefreshFixture(t *testing.T) {
	gunit.Run(new(AccessTokenRefreshFixture), t)
}

type AccessTokenRefreshFixture struct {
	*gunit.Fixture
	start time.Time
}

func (this *AccessTokenRefreshFixture) Setup() {
	this.start = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

func (this *AccessTokenRefreshFixture) TestShortRunsDoNotRefresh() {
	this.So(needsAccessTokenRefresh(this.start, this.start.Add(2*time.Second)), should.BeFalse)
	this.So(needsAccessTokenRefresh(this.start, this.start.Add(29*time.Minute)), should.BeFalse)
}

func (this *AccessTokenRefreshFixture) TestLongRunsRefresh() {
	this.So(needsAccessTokenRefresh(this.start, this.start.Add(31*time.Minute)), should.BeTrue)
}
