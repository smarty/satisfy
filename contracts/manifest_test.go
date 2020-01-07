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

func (this *ManifestFixture) Setup() {
}

func (this *ManifestFixture) TestMarshalManifest() {
    created, _ := time.Parse(time.RFC3339, "2020-01-07T23:12:02Z")
    original :=  Manifest{Name: "bowling-game",
        Version: "1.2.3",
        Created: created,
        Contents: []FileInfo{
            FileInfo{Path: "bowling.go", Size: 919, MD5Checksum: []uint8{186, 229, 122, 137, 125, 110, 10, 254, 227, 149, 39, 139, 184, 110, 49, 246}, Permissions: 420},
            FileInfo{Path: "bowling_test.go", Size: 1221, MD5Checksum: []uint8{232, 132, 144, 94, 153, 92, 245, 189, 40, 59, 17, 246, 224, 168, 218, 11}, Permissions: 420},
            FileInfo{Path: "go.mod", Size: 120, MD5Checksum: []uint8{245, 65, 127, 107, 204, 138, 143, 134, 5, 150, 39, 219, 234, 152, 129, 50}, Permissions: 384},
            FileInfo{Path: "go.sum", Size: 368, MD5Checksum: []uint8{38, 190, 140, 38, 125, 97, 144, 66, 216, 158, 151, 56, 233, 51, 237, 147}, Permissions: 384},
            FileInfo{Path: "stuff/hello.txt", Size: 6, MD5Checksum: []uint8{9, 247, 224, 47, 18, 144, 190, 33, 29, 167, 7, 162, 102, 241, 83, 179}, Permissions: 420},
        },
    }
    raw, err  := json.Marshal(original)
    this.So(err, should.BeNil)
    var clone Manifest
    err = json.Unmarshal(raw, &clone)
    this.So(err, should.BeNil)
    this.So(clone, should.Resemble, original)
}
