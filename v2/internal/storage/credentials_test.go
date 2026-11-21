package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gcs"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2/internal/model"
)

func TestRefreshingCredentialsFixture(t *testing.T) {
	gunit.Run(new(RefreshingCredentialsFixture), t)
}

type RefreshingCredentialsFixture struct {
	*gunit.Fixture
	reader *fakeCredentialsReader
	source *RefreshingCredentials
	now    time.Time
	ctx    context.Context
	maxAge time.Duration
}

func (this *RefreshingCredentialsFixture) Setup() {
	this.ctx = context.Background()
	this.reader = &fakeCredentialsReader{}
	this.maxAge = time.Minute
	this.now = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	this.source = NewRefreshingCredentials(this.reader, "token", this.maxAge)
	this.source.now = func() time.Time { return this.now }
}

func (this *RefreshingCredentialsFixture) TestFirstUseReadsCredentials() {
	credentials, err := this.source.Credentials(this.ctx)
	this.So(err, should.BeNil)
	this.So(credentials.BearerToken, should.Equal, "Bearer 1")
	this.So(this.reader.values, should.Resemble, []string{"token"})
}

func (this *RefreshingCredentialsFixture) TestFreshCredentialsAreReused() {
	_, _ = this.source.Credentials(this.ctx)
	this.now = this.now.Add(this.maxAge - time.Second)
	credentials, _ := this.source.Credentials(this.ctx)
	this.So(credentials.BearerToken, should.Equal, "Bearer 1")
	this.So(this.reader.values, should.HaveLength, 1)
}

func (this *RefreshingCredentialsFixture) TestStaleCredentialsAreReadAgain() {
	_, _ = this.source.Credentials(this.ctx)
	this.now = this.now.Add(this.maxAge)
	credentials, _ := this.source.Credentials(this.ctx)
	this.So(credentials.BearerToken, should.Equal, "Bearer 2")
}

func (this *RefreshingCredentialsFixture) TestFirstReadFailureIsReturnedAndRetriedNextTime() {
	failure := errors.New("vault unavailable")
	this.reader.err = failure
	_, err := this.source.Credentials(this.ctx)
	this.So(err, should.Equal, failure) // not retryable: the source may not be configured at all

	this.reader.err = nil
	credentials, err := this.source.Credentials(this.ctx)
	this.So(err, should.BeNil)
	this.So(credentials.BearerToken, should.Equal, "Bearer 2")
}

func (this *RefreshingCredentialsFixture) TestRefreshFailureKeepsUsingYoungEnoughCredentials() {
	_, _ = this.source.Credentials(this.ctx)
	this.reader.err = errors.New("vault unavailable")
	this.now = this.now.Add(this.maxAge*3/2 - time.Second)

	credentials, err := this.source.Credentials(this.ctx)
	this.So(err, should.BeNil)
	this.So(credentials.BearerToken, should.Equal, "Bearer 1")

	this.reader.err = nil
	credentials, _ = this.source.Credentials(this.ctx)
	this.So(credentials.BearerToken, should.Equal, "Bearer 3") // a read is attempted every time
}

func (this *RefreshingCredentialsFixture) TestRefreshFailureOfOldCredentialsIsRetryable() {
	_, _ = this.source.Credentials(this.ctx)
	failure := errors.New("vault unavailable")
	this.reader.err = failure
	this.now = this.now.Add(this.maxAge * 3 / 2)

	_, err := this.source.Credentials(this.ctx)
	this.So(errors.Is(err, failure), should.BeTrue)
	this.So(errors.Is(err, model.ErrRetryable), should.BeTrue)
}

func (this *RefreshingCredentialsFixture) TestInvalidatedCredentialsAreNeverReused() {
	_, _ = this.source.Credentials(this.ctx)
	this.source.Invalidate()
	this.reader.err = errors.New("vault unavailable")

	credentials, err := this.source.Credentials(this.ctx)
	this.So(errors.Is(err, model.ErrRetryable), should.BeTrue)
	this.So(credentials, should.Resemble, gcs.Credentials{})
}

func (this *RefreshingCredentialsFixture) TestStaticCredentials() {
	credentials, err := StaticCredentials{BearerToken: "Bearer x"}.Credentials(this.ctx)
	this.So(err, should.BeNil)
	this.So(credentials.BearerToken, should.Equal, "Bearer x")
}

type fakeCredentialsReader struct {
	values []string
	err    error
}

func (this *fakeCredentialsReader) Read(_ context.Context, value string) (gcs.Credentials, error) {
	this.values = append(this.values, value)
	if this.err != nil {
		return gcs.Credentials{}, this.err
	}
	return gcs.Credentials{BearerToken: "Bearer " + string(rune('0'+len(this.values)))}, nil
}
