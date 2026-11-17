package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2/contracts"
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
	this.client = NewRetryClient(this.fakeClient, 4, DefaultRetryDelay)
	this.client.wait = func(_ context.Context, duration time.Duration) error {
		this.naps = append(this.naps, duration)
		return nil
	}
}

func (this *RetryFixture) TestUploadCallsInner() {
	sent := contracts.UploadRequest{ContentType: "test"}

	err := this.client.Upload(context.Background(), sent)

	this.So(err, should.BeNil)
	this.So(this.fakeClient.uploadRequest.ContentType, should.Equal, "test")
}

func (this *RetryFixture) TestUploadRetryOnError() {
	this.fakeClient.error = aRetryError

	err := this.client.Upload(context.Background(), contracts.UploadRequest{})

	this.So(err, should.Equal, aRetryError)
	this.So(this.fakeClient.uploadAttempts, should.Equal, 5)
	this.So(this.naps, should.Resemble, []time.Duration{
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
	})
}

func (this *RetryFixture) TestUploadBodyIsRewoundBeforeEveryAttempt() {
	this.fakeClient.error = aRetryError

	_ = this.client.Upload(context.Background(), contracts.UploadRequest{Body: strings.NewReader("payload")})

	this.So(this.fakeClient.uploadBodies, should.Resemble, []string{"payload", "payload", "payload", "payload", "payload"})
}

func (this *RetryFixture) TestCancelledContextStopsRetries() {
	this.fakeClient.error = aRetryError
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	this.client.wait = waitContext

	_, err := this.client.Download(ctx, url.URL{})

	this.So(err, should.Equal, context.Canceled)
	this.So(this.fakeClient.downloadAttempts, should.Equal, 1)
}

func (this *RetryFixture) TestWaitContextWaitsForTheDelay() {
	this.So(waitContext(context.Background(), time.Millisecond), should.BeNil)
}

func (this *RetryFixture) TestUploadNoRetryOnRegularErrors() {
	this.fakeClient.error = aRegularError

	err := this.client.Upload(context.Background(), contracts.UploadRequest{})

	this.So(err, should.Equal, aRegularError)
	this.So(this.fakeClient.uploadAttempts, should.Equal, 1)
	this.So(this.naps, should.BeEmpty)
}

func (this *RetryFixture) TestDownloadCallsInner() {
	this.fakeClient.downloadContent = "content"
	request := url.URL{Host: "host.com"}

	reader, err := this.client.Download(context.Background(), request)

	all, _ := io.ReadAll(reader)
	this.So(string(all), should.Equal, "content")
	this.So(err, should.BeNil)
	this.So(this.fakeClient.downloadRequest, should.Resemble, request)
}

func (this *RetryFixture) TestDownloadRetryOnError() {
	this.fakeClient.error = aRetryError

	_, err := this.client.Download(context.Background(), url.URL{})

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

	body, err := this.client.Download(context.Background(), url.URL{})

	this.So(body, should.BeNil)
	this.So(err, should.Equal, aRegularError)
	this.So(this.fakeClient.downloadAttempts, should.Equal, 1)
	this.So(this.naps, should.BeEmpty)
}

var (
	aRetryError   = fmt.Errorf("this is a retry error %w", contracts.ErrRetryable)
	aRegularError = errors.New("this is a regular error")
)

/////////////////////////////////////////////////////////////////////////////////

type FakeClient struct {
	uploadRequest  contracts.UploadRequest
	uploadAttempts int
	uploadBodies   []string

	downloadRequest  url.URL
	downloadContent  string
	downloadAttempts int

	error error
}

func (this *FakeClient) Download(_ context.Context, request url.URL) (io.ReadCloser, error) {
	this.downloadRequest = request
	this.downloadAttempts++
	return io.NopCloser(strings.NewReader(this.downloadContent)), this.error
}

func (this *FakeClient) Seek(_ context.Context, request url.URL, begin, end int64) (io.ReadCloser, error) {
	this.downloadRequest = request
	this.downloadAttempts++
	return io.NopCloser(strings.NewReader(this.downloadContent[begin:end])), this.error
}

func (this *FakeClient) Size(_ context.Context, request url.URL) (int64, error) {
	return int64(len(this.downloadContent)), this.error
}

//TODO: Implement tests for seek and size

func (this *FakeClient) Upload(_ context.Context, request contracts.UploadRequest) error {
	this.uploadRequest = request
	this.uploadAttempts++
	if request.Body != nil {
		raw, _ := io.ReadAll(request.Body)
		this.uploadBodies = append(this.uploadBodies, string(raw))
	}
	return this.error
}
