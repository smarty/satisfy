package contracts

import (
	"net/url"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
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
	pointer := &url
	raw, err := pointer.MarshalJSON()
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