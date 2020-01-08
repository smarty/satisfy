package contracts

import (
	"encoding/json"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
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
