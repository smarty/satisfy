package core

import (
	"errors"
	"io"
	"io/ioutil"
	"net/url"
	"strings"
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
	client     *RetryClient
	fakeClient *FakeClient
}

func (this *RetryFixture) Setup() {
	this.fakeClient = &FakeClient{}
	this.client = NewRetryClient(this.fakeClient, 4)
	this.client.sleeper = clock.StayAwake()
	this.client.logger = logging.Capture()
}

func (this *RetryFixture) TestUploadCallsInner() {
	sent := contracts.UploadRequest{ContentType: "test"}

	err := this.client.Upload(sent)

	this.So(err, should.BeNil)
	this.So(this.fakeClient.uploadRequest.ContentType, should.Equal, "test")
}

func (this *RetryFixture) TestUploadRetryOnError() {
	this.fakeClient.error = anError

	err := this.client.Upload(contracts.UploadRequest{})

	this.So(err, should.Equal, anError)
	this.So(this.fakeClient.uploadAttempts, should.Equal, 5)
	this.So(this.client.sleeper.Naps, should.Resemble, []time.Duration{
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
	})
}

func (this *RetryFixture) TestDownloadCallsInner() {
	this.fakeClient.downloadContent = "content"
	request := url.URL{Host: "host.com"}

	reader, err := this.client.Download(request)

	all, _ := ioutil.ReadAll(reader)
	this.So(string(all), should.Equal, "content")
	this.So(err, should.BeNil)
	this.So(this.fakeClient.downloadRequest, should.Resemble, request)
}

func (this *RetryFixture) TestDownloadRetryOnError() {
	this.fakeClient.error = anError

	_, err := this.client.Download(url.URL{})

	this.So(err, should.Equal, anError)
	this.So(this.fakeClient.downloadAttempts, should.Equal, 5)
	this.So(this.client.sleeper.Naps, should.Resemble, []time.Duration{
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
	})
}

var anError = errors.New("this is an error")

/////////////////////////////////////////////////////////////////////////////////

type FakeClient struct {
	uploadRequest  contracts.UploadRequest
	uploadAttempts int

	downloadRequest  url.URL
	downloadContent  string
	downloadAttempts int

	error error
}

func (this *FakeClient) Download(request url.URL) (io.ReadCloser, error) {
	this.downloadRequest = request
	this.downloadAttempts++
	return ioutil.NopCloser(strings.NewReader(this.downloadContent)), this.error
}

func (this *FakeClient) Upload(request contracts.UploadRequest) error {
	this.uploadRequest = request
	this.uploadAttempts++
	return this.error
}
