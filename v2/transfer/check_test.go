package transfer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2/contracts"
)

func TestCheckAppFixture(t *testing.T) {
	gunit.Run(new(CheckAppFixture), t)
}

type CheckAppFixture struct {
	*gunit.Fixture
	client *fakeSizer
	app    *CheckApp
}

func (this *CheckAppFixture) Setup() {
	this.client = &fakeSizer{}
	config := contracts.UploadConfig{PackageConfig: contracts.PackageConfig{
		PackageName:         "name",
		PackageVersion:      "1.2.3",
		RemoteAddressPrefix: &contracts.URL{Scheme: "gcs", Host: "bucket"},
	}}
	this.app = &CheckApp{config: config, client: this.client}
}

func (this *CheckAppFixture) TestMissingPackageIsNotAnError() {
	this.client.err = contracts.NewStatusCodeError(http.StatusNotFound, []int{http.StatusOK}, url.URL{})
	this.So(this.app.Run(context.Background()), should.BeNil)
	this.So(this.client.address.Path, should.Equal, "/name/1.2.3/manifest.json")
}

func (this *CheckAppFixture) TestExistingPackageReturnsErrPackageExists() {
	this.So(errors.Is(this.app.Run(context.Background()), contracts.ErrPackageExists), should.BeTrue)
}

func (this *CheckAppFixture) TestOtherFailuresAreReturned() {
	failure := errors.New("boom")
	this.client.err = failure
	this.So(errors.Is(this.app.Run(context.Background()), failure), should.BeTrue)
}

func (this *CheckAppFixture) TestOverwriteSkipsTheCheck() {
	this.app.config.Overwrite = true
	this.So(this.app.Run(context.Background()), should.BeNil)
	this.So(this.client.calls, should.Equal, 0)
}

type fakeSizer struct {
	address url.URL
	err     error
	calls   int
}

func (this *fakeSizer) Size(_ context.Context, address url.URL) (int64, error) {
	this.calls++
	this.address = address
	return 0, this.err
}
func (this *fakeSizer) Download(context.Context, url.URL) (io.ReadCloser, error) { panic("not used") }
func (this *fakeSizer) Seek(context.Context, url.URL, int64, int64) (io.ReadCloser, error) {
	panic("not used")
}
