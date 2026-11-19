package satisfy

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2/internal/model"
)

func TestStatusCodeFixture(t *testing.T) {
	gunit.Run(new(StatusCodeFixture), t)
}

type StatusCodeFixture struct {
	*gunit.Fixture
}

func statusError(code int) error {
	return model.NewStatusCodeError(code, []int{http.StatusOK}, url.URL{Scheme: "gcs", Host: "bucket"})
}

func (this *StatusCodeFixture) TestNoStatusError() {
	code, found := StatusCode(nil)
	this.So(code, should.Equal, 0)
	this.So(found, should.BeFalse)

	code, found = StatusCode(errors.New("other"))
	this.So(code, should.Equal, 0)
	this.So(found, should.BeFalse)
}

func (this *StatusCodeFixture) TestWrappedStatusError() {
	code, found := StatusCode(fmt.Errorf("context: %w", statusError(http.StatusForbidden)))
	this.So(code, should.Equal, http.StatusForbidden)
	this.So(found, should.BeTrue)
}

func (this *StatusCodeFixture) TestJoinedErrorsReportTheFirstStatusError() {
	code, found := StatusCode(errors.Join(errors.New("other"), statusError(http.StatusForbidden), statusError(http.StatusNotFound)))
	this.So(code, should.Equal, http.StatusForbidden)
	this.So(found, should.BeTrue)
}
