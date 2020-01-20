package remote

import (
	"errors"
	"io"
	"net/url"
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
	fakeClient    *FakeClient
}

func (this *RetryFixture) Setup() {
	this.fakeClient = &FakeClient{}
	this.retryUploader = NewRetryClient(this.fakeClient, 4)
	this.retryUploader.sleeper = clock.StayAwake()
	this.retryUploader.logger = logging.Capture()
}

func (this *RetryFixture) TestUploadCallsInner() {
	sent := contracts.UploadRequest{ContentType: "test"}

	err := this.retryUploader.Upload(sent)

	this.So(err, should.BeNil)
	this.So(this.fakeClient.received.ContentType, should.Equal, "test")
}

var anError = errors.New("this is an error")

func (this *RetryFixture) TestRetryOnError() {
	this.fakeClient.error = anError
	err := this.retryUploader.Upload(contracts.UploadRequest{})
	this.So(err, should.Equal, anError)
	this.So(this.fakeClient.attempts, should.Equal, 5)
	this.So(this.retryUploader.sleeper.Naps, should.Resemble, []time.Duration{
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
	})
}

/////////////////////////////////////////////////////////////////////////////////

type FakeClient struct {
	received contracts.UploadRequest
	error    error
	attempts int
}

func (this *FakeClient) Download(url.URL) (io.ReadCloser, error) {
	panic("implement me")
}

func (this *FakeClient) Upload(request contracts.UploadRequest) error {
	this.received = request
	this.attempts++
	return this.error
}
