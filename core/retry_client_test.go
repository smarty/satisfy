package core

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/contracts"
)

func TestRetryFixture(t *testing.T) {
	gunit.Run(new(RetryFixture), t)
}

type RetryFixture struct {
	*gunit.Fixture
	client     *RetryClient
	fakeClient *FakeClient
	naps       []time.Duration
}

func (this *RetryFixture) Setup() {
	this.fakeClient = &FakeClient{}
	this.client = NewRetryClient(this.fakeClient, 4, func(duration time.Duration) {
		this.naps = append(this.naps, duration)
	})
}

func (this *RetryFixture) TestUploadCallsInner() {
	sent := contracts.UploadRequest{ContentType: "test"}

	err := this.client.Upload(sent)

	this.So(err, should.BeNil)
	this.So(this.fakeClient.uploadRequest.ContentType, should.Equal, "test")
}

func (this *RetryFixture) TestUploadRetryOnError() {
	this.fakeClient.error = aRetryError

	err := this.client.Upload(contracts.UploadRequest{})

	this.So(err, should.Equal, aRetryError)
	this.So(this.fakeClient.uploadAttempts, should.Equal, 5)
	this.So(this.naps, should.Resemble, []time.Duration{
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
	})
}

func (this *RetryFixture) TestUploadNoRetryOnRegularErrors() {
	this.fakeClient.error = aRegularError

	err := this.client.Upload(contracts.UploadRequest{})

	this.So(err, should.Equal, aRegularError)
	this.So(this.fakeClient.uploadAttempts, should.Equal, 1)
	this.So(this.naps, should.BeEmpty)
}

func (this *RetryFixture) TestDownloadCallsInner() {
	this.fakeClient.downloadContent = "content"
	request := url.URL{Host: "host.com"}

	reader, err := this.client.Download(request)

	all, _ := io.ReadAll(reader)
	this.So(string(all), should.Equal, "content")
	this.So(err, should.BeNil)
	this.So(this.fakeClient.downloadRequest, should.Resemble, request)
}

func (this *RetryFixture) TestDownloadRetryOnError() {
	this.fakeClient.error = aRetryError

	_, err := this.client.Download(url.URL{})

	this.So(err, should.Equal, aRetryError)
	this.So(this.fakeClient.downloadAttempts, should.Equal, 5)
	this.So(this.naps, should.Resemble, []time.Duration{
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
	})
}
func (this *RetryFixture) TestDownloadNoRetryOnRegularErrors() {
	this.fakeClient.error = aRegularError

	body, err := this.client.Download(url.URL{})

	this.So(body, should.BeNil)
	this.So(err, should.Equal, aRegularError)
	this.So(this.fakeClient.downloadAttempts, should.Equal, 1)
	this.So(this.naps, should.BeEmpty)
}

var (
	aRetryError   = fmt.Errorf("this is a retry error %w", contracts.RetryErr)
	aRegularError = errors.New("this is a regular error")
)

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
	return io.NopCloser(strings.NewReader(this.downloadContent)), this.error
}

func (this *FakeClient) Seek(request url.URL, begin, end int64) (io.ReadCloser, error) {
	this.downloadRequest = request
	this.downloadAttempts++
	return io.NopCloser(strings.NewReader(this.downloadContent[begin:end])), this.error
}

func (this *FakeClient) Size(request url.URL) (int64, error) {
	return int64(len(this.downloadContent)), this.error
}

//TODO: Implement tests for seek and size

func (this *FakeClient) Upload(request contracts.UploadRequest) error {
	this.uploadRequest = request
	this.uploadAttempts++
	return this.error
}
