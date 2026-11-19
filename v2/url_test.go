package satisfy

import (
	"errors"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestParseURLFixture(t *testing.T) {
	gunit.Run(new(ParseURLFixture), t)
}

type ParseURLFixture struct {
	*gunit.Fixture
}

func (this *ParseURLFixture) TestValidAddresses() {
	for raw, expected := range map[string]URL{
		"gcs://bucket":                {Scheme: "gcs", Host: "bucket"},
		"gcs://bucket/":               {Scheme: "gcs", Host: "bucket"},
		"gcs://bucket/releases/2026/": {Scheme: "gcs", Host: "bucket", Path: "/releases/2026"},
		"bucket":                      {Scheme: "gcs", Host: "bucket"},
		" bucket/releases ":           {Scheme: "gcs", Host: "bucket", Path: "/releases"},
		"/bucket/releases":            {Scheme: "gcs", Host: "bucket", Path: "/releases"},
	} {
		actual, err := ParseURL(raw)
		this.So(err, should.BeNil)
		this.So(actual, should.Resemble, expected)
	}
}

func (this *ParseURLFixture) TestInvalidAddresses() {
	for _, raw := range []string{"", "gcs://", "gcs:///path", "https://storage.googleapis.com/bucket", "gcs://bu cket"} {
		_, err := ParseURL(raw)
		this.So(errors.Is(err, ErrInvalidConfig), should.BeTrue)
	}
}

func (this *ParseURLFixture) TestString() {
	address, _ := ParseURL("bucket/releases")
	this.So(address.String(), should.Equal, "gcs://bucket/releases")
}
