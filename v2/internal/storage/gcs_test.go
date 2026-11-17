package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gcs"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2/internal/model"
)

func TestGoogleCloudStorageClientFixture(t *testing.T) {
	gunit.Run(new(GoogleCloudStorageClientFixture), t)
}

type GoogleCloudStorageClientFixture struct {
	*gunit.Fixture
	transport *fakeTransport
	client    *GoogleCloudStorageClient
	address   url.URL
}

func (this *GoogleCloudStorageClientFixture) Setup() {
	this.transport = &fakeTransport{status: http.StatusOK, header: http.Header{}}
	credentials := StaticCredentials(gcs.Credentials{BearerToken: "Bearer token"})
	this.client = NewGoogleCloudStorageClient(&http.Client{Transport: this.transport}, credentials, []int{http.StatusOK})
	this.address = url.URL{Scheme: "gcs", Host: "bucket", Path: "/package/manifest.json"}
}

func (this *GoogleCloudStorageClientFixture) upload(ifGenerationMatch *int64) error {
	return this.client.Upload(context.Background(), model.UploadRequest{
		RemoteAddress:     this.address,
		Body:              strings.NewReader("{}"),
		Size:              2,
		ContentType:       "application/json",
		Checksum:          []byte("0123456789abcdef"),
		IfGenerationMatch: ifGenerationMatch,
	})
}

func (this *GoogleCloudStorageClientFixture) TestConditionalUploadSendsGenerationPrecondition() {
	generation := int64(42)
	this.So(this.upload(&generation), should.BeNil)
	this.So(this.transport.request.Header.Get("x-goog-if-generation-match"), should.Equal, "42")
}

func (this *GoogleCloudStorageClientFixture) TestUnconditionalUploadSendsNoPrecondition() {
	this.So(this.upload(nil), should.BeNil)
	this.So(this.transport.request.Header.Values("x-goog-if-generation-match"), should.BeEmpty)
}

func (this *GoogleCloudStorageClientFixture) TestFailedPreconditionIsReported() {
	this.transport.status = http.StatusPreconditionFailed
	generation := int64(0)
	err := this.upload(&generation)
	this.So(errors.Is(err, model.ErrPreconditionFailed), should.BeTrue)
	this.So(errors.Is(err, model.ErrRetryable), should.BeFalse)
}

func (this *GoogleCloudStorageClientFixture) TestDownloadReportsObjectGeneration() {
	this.transport.header.Set("x-goog-generation", "1700000000000001")
	body, err := this.client.Download(context.Background(), this.address)
	this.So(err, should.BeNil)
	generational, ok := body.(model.Generational)
	this.So(ok, should.BeTrue)
	this.So(generational.Generation(), should.Equal, int64(1700000000000001))
}

func (this *GoogleCloudStorageClientFixture) TestTransportErrorsAreRetryableAndInspectable() {
	this.transport.err = &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}

	_, err := this.client.Download(context.Background(), this.address)

	this.So(errors.Is(err, model.ErrRetryable), should.BeTrue)
	this.So(errors.Is(err, syscall.ECONNREFUSED), should.BeTrue)
	_, isNetError := errors.AsType[net.Error](err)
	this.So(isNetError, should.BeTrue)
}

///////////////////////////////////////////////////////////////////////////////

type fakeTransport struct {
	err     error
	status  int
	header  http.Header
	request *http.Request
}

func (this *fakeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	this.request = request
	if this.err != nil {
		return nil, this.err
	}
	if request.Body != nil {
		_, _ = io.Copy(io.Discard, request.Body)
	}
	return &http.Response{
		StatusCode: this.status,
		Header:     this.header,
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Request:    request,
	}, nil
}
