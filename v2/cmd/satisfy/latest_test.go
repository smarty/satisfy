package main

import (
	"io"
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
	_, err := parseLatestConfig([]string{"-package", "p"}, io.Discard)
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "-bucket is required")
}

func (this *ParseLatestConfigFixture) TestBucketWithSlashIsRejected() {
	_, err := parseLatestConfig([]string{"-bucket", "my-bucket/extra", "-package", "p"}, io.Discard)
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "bare bucket name")
}

func (this *ParseLatestConfigFixture) TestBucketWithSchemeIsRejected() {
	_, err := parseLatestConfig([]string{"-bucket", "gs://my-bucket", "-package", "p"}, io.Discard)
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "bare bucket name")
}

func (this *ParseLatestConfigFixture) TestMissingPackageIsRejected() {
	_, err := parseLatestConfig([]string{"-bucket", "my-bucket"}, io.Discard)
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "-package is required")
}

func (this *ParseLatestConfigFixture) TestPackageOfOnlySlashesIsRejected() {
	_, err := parseLatestConfig([]string{"-bucket", "my-bucket", "-package", "///"}, io.Discard)
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "-package is required")
}
