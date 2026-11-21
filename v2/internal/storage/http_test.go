package storage

import (
	"net/http"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestHTTPClientFixture(t *testing.T) {
	gunit.Run(new(HTTPClientFixture), t)
}

type HTTPClientFixture struct {
	*gunit.Fixture
}

func (this *HTTPClientFixture) TestConnectionsAreReused() {
	transport := NewHTTPClient().Transport.(*http.Transport)

	this.So(transport.DisableKeepAlives, should.BeFalse)
	this.So(transport.ForceAttemptHTTP2, should.BeTrue)
	this.So(transport.MaxIdleConnsPerHost, should.Equal, maxIdleConnectionsPerHost)
	this.So(transport.Proxy, should.NotBeNil)
}
