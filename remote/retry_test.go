package remote

import (
	"errors"
	"testing"
	"time"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/clock"
	"github.com/smartystreets/gunit"
	"github.com/smartystreets/logging"
)

func TestRetryFixture(t *testing.T) {
	gunit.Run(new(RetryFixture), t)
}

type RetryFixture struct {
	*gunit.Fixture
	retryUploader    *RetryUploader
	received         contracts.UploadRequest
	innerUploadError error
	attempts         int
}

func (this *RetryFixture) Upload(request contracts.UploadRequest) error {
	this.received = request
	this.attempts++
	return this.innerUploadError
}

func (this *RetryFixture) Setup() {
	this.retryUploader = NewRetryUploader(this, 4)
	this.retryUploader.sleeper = clock.StayAwake()
	this.retryUploader.logger = logging.Capture()
}

func (this *RetryFixture) TestUploadCallsInner() {
	sent := contracts.UploadRequest{ContentType: "test"}
	err := this.retryUploader.Upload(sent)
	this.So(err, should.BeNil)
	this.So(this.received.ContentType, should.Equal, "test")
}

var innerError = errors.New("this is an innerUploadError")

func (this *RetryFixture) TestRetryOnError() {
	this.innerUploadError = innerError
	err := this.retryUploader.Upload(contracts.UploadRequest{})
	this.So(err, should.Equal, innerError)
	this.So(this.attempts, should.Equal, 5)
	this.So(this.retryUploader.sleeper.Naps, should.Resemble, []time.Duration{
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
	})
}