package remote

import (
	"errors"
	"testing"
	"time"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/clock"
	"github.com/smartystreets/gunit"
	"github.com/smartystreets/logging"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

func TestRetryFixture(t *testing.T) {
	gunit.Run(new(RetryFixture), t)
}

type RetryFixture struct {
	*gunit.Fixture
	retryUploader *RetryClient
	fakeUploader  *FakeUploader
}

func (this *RetryFixture) Setup() {
	this.fakeUploader = &FakeUploader{}
	this.retryUploader = NewRetryClient(this.fakeUploader, 4)
	this.retryUploader.sleeper = clock.StayAwake()
	this.retryUploader.logger = logging.Capture()
}

func (this *RetryFixture) TestUploadCallsInner() {
	sent := contracts.UploadRequest{ContentType: "test"}

	err := this.retryUploader.Upload(sent)

	this.So(err, should.BeNil)
	this.So(this.fakeUploader.received.ContentType, should.Equal, "test")
}

var anError = errors.New("this is an error")

func (this *RetryFixture) TestRetryOnError() {
	this.fakeUploader.error = anError
	err := this.retryUploader.Upload(contracts.UploadRequest{})
	this.So(err, should.Equal, anError)
	this.So(this.fakeUploader.attempts, should.Equal, 5)
	this.So(this.retryUploader.sleeper.Naps, should.Resemble, []time.Duration{
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
	})
}

/////////////////////////////////////////////////////////////////////////////////

type FakeUploader struct {
	received contracts.UploadRequest
	error    error
	attempts int
}

func (this *FakeUploader) Upload(request contracts.UploadRequest) error {
	this.received = request
	this.attempts++
	return this.error
}
