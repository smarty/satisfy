package core

import (
	"errors"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

func TestCompoundIntegrityCheckFixture(t *testing.T) {
	gunit.Run(new(CompoundIntegrityCheckFixture), t)
}

type CompoundIntegrityCheckFixture struct {
	*gunit.Fixture

	checker *CompoundIntegrityCheck
	innerA  *FakeIntegrityCheck
	innerB  *FakeIntegrityCheck
}

func (this *CompoundIntegrityCheckFixture) Setup() {
	this.innerA = &FakeIntegrityCheck{}
	this.innerB = &FakeIntegrityCheck{}
	this.checker = NewCompoundIntegrityCheck(this.innerA, this.innerB)
}

func (this *CompoundIntegrityCheckFixture) TestAllInnerIntegrityTestsPass() {
	this.So(this.checker.Verify(contracts.Manifest{}), should.BeNil)
}

func (this *CompoundIntegrityCheckFixture) TestAnyIntegrityTestsFail() {
	this.innerB.err = errors.New("test")

	this.So(this.checker.Verify(contracts.Manifest{}), should.NotBeNil)
}

//////////////////////////////////////////////////////////////////////

type FakeIntegrityCheck struct {
	err error
}

func (this *FakeIntegrityCheck) Verify(manifest contracts.Manifest) error {
	return this.err
}
