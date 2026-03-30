package core

import (
	"errors"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/internal/plumbing"
)

func TestCompoundIntegrityCheckFixture(t *testing.T) {
	gunit.Run(new(CompoundIntegrityCheckFixture), t)
}

type CompoundIntegrityCheckFixture struct {
	*gunit.Fixture

	checker   *CompoundIntegrityCheck
	innerA    *FakeIntegrityCheck
	innerB    *FakeIntegrityCheck
	manifest  plumbing.Manifest
	localPath string
}

func (this *CompoundIntegrityCheckFixture) Setup() {
	this.innerA = &FakeIntegrityCheck{}
	this.innerB = &FakeIntegrityCheck{}
	this.checker = NewCompoundIntegrityCheck(this.innerA, this.innerB)
	this.manifest = plumbing.Manifest{Name: "package-name"}
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
	manifest  plumbing.Manifest
	localPath string
}

func (this *FakeIntegrityCheck) Verify(manifest plumbing.Manifest, localPath string) error {
	this.manifest = manifest
	this.localPath = localPath
	return this.err
}
