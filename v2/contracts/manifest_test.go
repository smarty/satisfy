package contracts

import (
	"encoding/json"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestManifestFixture(t *testing.T) {
	gunit.Run(new(ManifestFixture), t)
}

type ManifestFixture struct {
	*gunit.Fixture
}

func (this *ManifestFixture) TestMarshalManifest() {
	original := Manifest{
		Name:    "package-name",
		Version: "1.2.3",
		Archive: Archive{
			Filename:    "filename",
			Size:        1,
			MD5Checksum: []byte("checksum"),
			Contents: []ArchiveItem{
				{Path: "item1", Size: 1, MD5Checksum: []byte("item1")},
				{Path: "item2", Size: 2, MD5Checksum: []byte("item2")},
			},
		},
	}
	clone := this.unmarshal(this.marshal(original))
	this.So(clone, should.Resemble, original)
}

func (this *ManifestFixture) TestMarshalManifestWithTags() {
	original := Manifest{
		Name:    "package-name",
		Version: "1.2.3",
		Tags: []Tag{
			{Name: "stable", Version: "1.2.2"},
			{Name: "experimental", Version: "1.2.3"},
		},
	}
	clone := this.unmarshal(this.marshal(original))
	this.So(clone, should.Resemble, original)
}

func (this *ManifestFixture) TestTagsOmittedFromJSONWhenEmpty() {
	raw := this.marshal(Manifest{Name: "package-name", Version: "1.2.3"})
	this.So(string(raw), should.NotContainSubstring, "tags")
}

func (this *ManifestFixture) TestTagVersion() {
	manifest := Manifest{
		Version: "1.2.3",
		Tags: []Tag{
			{Name: "stable", Version: "1.2.2"},
			{Name: "experimental", Version: "1.2.3"},
		},
	}

	version, found := manifest.TagVersion("stable")
	this.So(found, should.BeTrue)
	this.So(version, should.Equal, "1.2.2")

	version, found = manifest.TagVersion("nope")
	this.So(found, should.BeFalse)
	this.So(version, should.BeBlank)
}

func (this *ManifestFixture) unmarshal(raw []byte) Manifest {
	var clone Manifest
	err := json.Unmarshal(raw, &clone)
	this.So(err, should.BeNil)
	return clone
}

func (this *ManifestFixture) marshal(original Manifest) []byte {
	raw, err := json.Marshal(original)
	this.So(err, should.BeNil)
	return raw
}
