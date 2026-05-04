package transfer

import (
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestParseLatestConfigFixture(t *testing.T) {
	gunit.Run(new(ParseLatestConfigFixture), t)
}

type ParseLatestConfigFixture struct {
	*gunit.Fixture
}

func (this *ParseLatestConfigFixture) TestMissingBucketIsRejected() {
	_, err := ParseLatestConfig([]string{"-package", "p"})
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "-bucket is required")
}

func (this *ParseLatestConfigFixture) TestBucketWithSlashIsRejected() {
	_, err := ParseLatestConfig([]string{"-bucket", "my-bucket/extra", "-package", "p"})
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "bare bucket name")
}

func (this *ParseLatestConfigFixture) TestBucketWithSchemeIsRejected() {
	_, err := ParseLatestConfig([]string{"-bucket", "gs://my-bucket", "-package", "p"})
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "bare bucket name")
}

func (this *ParseLatestConfigFixture) TestMissingPackageIsRejected() {
	_, err := ParseLatestConfig([]string{"-bucket", "my-bucket"})
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "-package is required")
}

func (this *ParseLatestConfigFixture) TestPackageOfOnlySlashesIsRejected() {
	_, err := ParseLatestConfig([]string{"-bucket", "my-bucket", "-package", "///"})
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "-package is required")
}
