package core

import (
	"errors"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
	"github.com/smartystreets/satisfy/contracts"
)

func TestCompoundIntegrityCheckFixture(t *testing.T) {
	gunit.Run(new(CompoundIntegrityCheckFixture), t)
}

type CompoundIntegrityCheckFixture struct {
	*gunit.Fixture

	checker   *CompoundIntegrityCheck
	innerA    *FakeIntegrityCheck
	innerB    *FakeIntegrityCheck
	manifest  contracts.Manifest
	localPath string
}

func (this *CompoundIntegrityCheckFixture) Setup() {
	this.innerA = &FakeIntegrityCheck{}
	this.innerB = &FakeIntegrityCheck{}
	this.checker = NewCompoundIntegrityCheck(this.innerA, this.innerB)
	this.manifest = contracts.Manifest{Name: "package-name"}
	this.localPath = "/local"
}

func (this *CompoundIntegrityCheckFixture) TestAllInnerIntegrityTestsPass() {
	this.So(this.checker.Verify(this.manifest, this.localPath), should.BeNil)
}

func (this *CompoundIntegrityCheckFixture) TestAnyIntegrityTestsFail() {
	this.innerB.err = errors.New("test")

	this.So(this.checker.Verify(this.manifest, this.localPath), should.NotBeNil)
	this.So(this.innerA.manifest, should.Resemble, this.manifest)
	this.So(this.innerA.localPath, should.Equal, this.localPath)
	this.So(this.innerB.manifest, should.Resemble, this.manifest)
	this.So(this.innerB.localPath, should.Equal, this.localPath)
}

//////////////////////////////////////////////////////////////////////

type FakeIntegrityCheck struct {
	err       error
	manifest  contracts.Manifest
	localPath string
}

func (this *FakeIntegrityCheck) Verify(manifest contracts.Manifest, localPath string) error {
	this.manifest = manifest
	this.localPath = localPath
	return this.err
}
