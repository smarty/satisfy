package main

import (
	"io"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2"
)

func TestTagsListFixture(t *testing.T) {
	gunit.Run(new(TagsListFixture), t)
}

type TagsListFixture struct {
	*gunit.Fixture
}

func (this *TagsListFixture) TestMissingBucketIsRejected() {
	_, err := parseTagsListConfig([]string{"-package", "p"}, io.Discard)
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "-bucket is required")
}

func (this *TagsListFixture) TestBucketWithSlashIsRejected() {
	_, err := parseTagsListConfig([]string{"-bucket", "my-bucket/extra", "-package", "p"}, io.Discard)
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "bare bucket name")
}

func (this *TagsListFixture) TestBucketWithSchemeIsRejected() {
	_, err := parseTagsListConfig([]string{"-bucket", "gs://my-bucket", "-package", "p"}, io.Discard)
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "bare bucket name")
}

func (this *TagsListFixture) TestMissingPackageIsRejected() {
	_, err := parseTagsListConfig([]string{"-bucket", "my-bucket"}, io.Discard)
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "-package is required")
}

func (this *TagsListFixture) TestPackageOfOnlySlashesIsRejected() {
	_, err := parseTagsListConfig([]string{"-bucket", "my-bucket", "-package", "///"}, io.Discard)
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "-package is required")
}

func (this *TagsListFixture) TestFormatTagsTextIsSortedAndTabSeparated() {
	tags := []satisfy.Tag{
		{Name: "stable", Version: "2026.01.A"},
		{Name: "experimental", Version: "2026.01.B"},
	}

	output, err := formatTags(tags, false)

	this.So(err, should.BeNil)
	this.So(output, should.Equal, "experimental\t2026.01.B\nstable\t2026.01.A\n")
}

func (this *TagsListFixture) TestFormatTagsTextEmptyProducesNoOutput() {
	output, err := formatTags(nil, false)

	this.So(err, should.BeNil)
	this.So(output, should.BeBlank)
}

func (this *TagsListFixture) TestFormatTagsJSONIsSorted() {
	tags := []satisfy.Tag{
		{Name: "stable", Version: "2026.01.A"},
		{Name: "experimental", Version: "2026.01.B"},
	}

	output, err := formatTags(tags, true)

	this.So(err, should.BeNil)
	this.So(output, should.Equal, `[
  {
    "name": "experimental",
    "version": "2026.01.B"
  },
  {
    "name": "stable",
    "version": "2026.01.A"
  }
]
`)
}

func (this *TagsListFixture) TestFormatTagsJSONEmptyIsEmptyArray() {
	output, err := formatTags(nil, true)

	this.So(err, should.BeNil)
	this.So(output, should.Equal, "[]\n")
}

func (this *TagsListFixture) TestFormatTagsDoesNotMutateInput() {
	tags := []satisfy.Tag{
		{Name: "stable", Version: "2026.01.A"},
		{Name: "experimental", Version: "2026.01.B"},
	}

	_, _ = formatTags(tags, false)

	this.So(tags[0], should.Resemble, satisfy.Tag{Name: "stable", Version: "2026.01.A"})
}
