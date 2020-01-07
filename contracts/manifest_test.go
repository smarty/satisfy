package contracts

import (
	"encoding/json"
	"testing"
	"time"

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
    created, _ := time.Parse(time.RFC3339, "2020-01-07T23:12:02Z")
    original :=  Manifest{
        Name: "package-name",
        Version: "1.2.3",
        Created: created,
        Contents: []FileInfo{
            {Path: "a", Size: 1, MD5Checksum: []byte{11}, Permissions: 111},
            {Path: "b", Size: 2, MD5Checksum: []byte{22}, Permissions: 222},
            {Path: "c", Size: 3, MD5Checksum: []byte{33}, Permissions: 333},
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
