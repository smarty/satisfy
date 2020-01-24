package cmd

import (
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
)

func TestFilterFixture(t *testing.T) {
	gunit.Run(new(FilterFixture), t)
}

type FilterFixture struct {
	*gunit.Fixture
	listing []Dependency
	filter  []string
}

func (this *FilterFixture) Setup() {
	this.appendDependency("A")
	this.appendDependency("B")
	this.appendDependency("C")
	this.appendDependency("A")
}

func (this *FilterFixture) TestEmptyFilter() {
	filtered := Filter(this.listing, this.filter)
	this.So(filtered, should.Resemble, this.listing)
}

func (this *FilterFixture) TestValidFilter() {
	filtered := Filter(this.listing, []string{"B"})
	this.So(filtered, should.Resemble, []Dependency{{PackageName: "B"}})
}

func (this *FilterFixture) TestMultipleMatchesOnPackageName() {
	filtered := Filter(this.listing, []string{"A"})
	this.So(filtered, should.Resemble, []Dependency{{PackageName: "A"}, {PackageName: "A"}})
}

func (this *FilterFixture) appendDependency(name string) {
	this.listing = append(this.listing, Dependency{PackageName: name})
}

func (this *FilterFixture) appendFilter(name string) {
	this.filter = append(this.filter, name)
}
