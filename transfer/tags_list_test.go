package transfer

import (
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/contracts"
)

func TestTagsListFixture(t *testing.T) {
	gunit.Run(new(TagsListFixture), t)
}

type TagsListFixture struct {
	*gunit.Fixture
}

func (this *TagsListFixture) TestMissingBucketIsRejected() {
	_, err := ParseTagsListConfig([]string{"-package", "p"})
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "-bucket is required")
}

func (this *TagsListFixture) TestBucketWithSlashIsRejected() {
	_, err := ParseTagsListConfig([]string{"-bucket", "my-bucket/extra", "-package", "p"})
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "bare bucket name")
}

func (this *TagsListFixture) TestBucketWithSchemeIsRejected() {
	_, err := ParseTagsListConfig([]string{"-bucket", "gs://my-bucket", "-package", "p"})
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "bare bucket name")
}

func (this *TagsListFixture) TestMissingPackageIsRejected() {
	_, err := ParseTagsListConfig([]string{"-bucket", "my-bucket"})
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "-package is required")
}

func (this *TagsListFixture) TestPackageOfOnlySlashesIsRejected() {
	_, err := ParseTagsListConfig([]string{"-bucket", "my-bucket", "-package", "///"})
	this.So(err, should.NotBeNil)
	this.So(err.Error(), should.ContainSubstring, "-package is required")
}

func (this *TagsListFixture) TestFormatTagsTextIsSortedAndTabSeparated() {
	tags := []contracts.Tag{
		{Name: "stable", Version: "2026.01.A"},
		{Name: "experimental", Version: "2026.01.B"},
	}

	output, err := FormatTags(tags, false)

	this.So(err, should.BeNil)
	this.So(output, should.Equal, "experimental\t2026.01.B\nstable\t2026.01.A\n")
}

func (this *TagsListFixture) TestFormatTagsTextEmptyProducesNoOutput() {
	output, err := FormatTags(nil, false)

	this.So(err, should.BeNil)
	this.So(output, should.BeBlank)
}

func (this *TagsListFixture) TestFormatTagsJSONIsSorted() {
	tags := []contracts.Tag{
		{Name: "stable", Version: "2026.01.A"},
		{Name: "experimental", Version: "2026.01.B"},
	}

	output, err := FormatTags(tags, true)

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
	output, err := FormatTags(nil, true)

	this.So(err, should.BeNil)
	this.So(output, should.Equal, "[]\n")
}

func (this *TagsListFixture) TestFormatTagsDoesNotMutateInput() {
	tags := []contracts.Tag{
		{Name: "stable", Version: "2026.01.A"},
		{Name: "experimental", Version: "2026.01.B"},
	}

	_, _ = FormatTags(tags, false)

	this.So(tags[0], should.Resemble, contracts.Tag{Name: "stable", Version: "2026.01.A"})
}
