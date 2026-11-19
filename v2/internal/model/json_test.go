package model

import (
	"encoding/json/v2"
	"net/url"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestURLFixture(t *testing.T) {
	gunit.Run(new(URLFixture), t)
}

type URLFixture struct {
	*gunit.Fixture
}

func (this *URLFixture) Setup() {
}

func (this *URLFixture) TestMarshal() {
	address, err := url.Parse("https://google.com")
	this.So(err, should.BeNil)
	url := URL(*address)
	raw, err := url.MarshalJSON()
	this.So(err, should.BeNil)
	this.So(string(raw), should.Equal, `"https://google.com"`)
}

func (this *URLFixture) TestUnmarshal() {
	raw := []byte(`"https://google.com"`)
	address := new(URL)
	err := address.UnmarshalJSON(raw)

	this.So(err, should.BeNil)
	this.So(address.Value().String(), should.Equal, "https://google.com")
}

func (this *URLFixture) TestUnmarshalNull() {
	raw := []byte(`"null"`)
	address := new(URL)
	err := address.UnmarshalJSON(raw)

	this.So(err, should.BeNil)
	this.So(address, should.Resemble, new(URL))
}

func (this *URLFixture) TestUnmarshalMalformedURL() {
	raw := []byte(`"%%%%%%"`)
	address := new(URL)
	err := address.UnmarshalJSON(raw)

	this.So(err, should.NotBeNil)
	this.So(address, should.Resemble, new(URL))
}

func (this *URLFixture) TestUnmarshalLiteralNullLeavesURLUnchanged() {
	address := new(URL)
	this.So(json.Unmarshal([]byte(`null`), address), should.BeNil)
	this.So(address, should.Resemble, new(URL))
}

func (this *URLFixture) TestUnmarshalDecodesJSONEscapes() {
	address := new(URL)
	this.So(json.Unmarshal([]byte(`"gcs:\/\/bucket\u002Fpath"`), address), should.BeNil)
	this.So(address.Value().String(), should.Equal, "gcs://bucket/path")
}

func (this *URLFixture) TestUnmarshalRejectsNonString() {
	address := new(URL)
	this.So(json.Unmarshal([]byte(`42`), address), should.NotBeNil)
}

func (this *URLFixture) TestMarshalEscapesAndWorksWithinValues() {
	dependency := Dependency{PackageName: "p", RemoteAddress: URL{Scheme: "gcs", Host: "bucket", Path: `/a"b`}}
	raw, err := json.Marshal(dependency) // by value: the URL field is not addressable
	this.So(err, should.BeNil)

	var decoded Dependency
	this.So(json.Unmarshal(raw, &decoded), should.BeNil)
	this.So(decoded.RemoteAddress, should.Resemble, dependency.RemoteAddress)
}
