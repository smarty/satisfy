package packaging

import (
	"context"
	"errors"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2/internal/model"
)

func TestCompoundIntegrityCheckFixture(t *testing.T) {
	gunit.Run(new(CompoundIntegrityCheckFixture), t)
}

type CompoundIntegrityCheckFixture struct {
	*gunit.Fixture

	checker   *CompoundIntegrityCheck
	innerA    *FakeIntegrityCheck
	innerB    *FakeIntegrityCheck
	manifest  model.Manifest
	localPath string
}

func (this *CompoundIntegrityCheckFixture) Setup() {
	this.innerA = &FakeIntegrityCheck{}
	this.innerB = &FakeIntegrityCheck{}
	this.checker = NewCompoundIntegrityCheck(this.innerA, this.innerB)
	this.manifest = model.Manifest{Name: "package-name"}
	this.localPath = "/local"
}

func (this *CompoundIntegrityCheckFixture) TestAllInnerIntegrityTestsPass() {
	this.So(this.checker.Verify(context.Background(), this.manifest, this.localPath), should.BeNil)
}

func (this *CompoundIntegrityCheckFixture) TestAnyIntegrityTestsFail() {
	this.innerB.err = errors.New("test")

	this.So(this.checker.Verify(context.Background(), this.manifest, this.localPath), should.NotBeNil)
	this.So(this.innerA.manifest, should.Resemble, this.manifest)
	this.So(this.innerA.localPath, should.Equal, this.localPath)
	this.So(this.innerB.manifest, should.Resemble, this.manifest)
	this.So(this.innerB.localPath, should.Equal, this.localPath)
}

//////////////////////////////////////////////////////////////////////

type FakeIntegrityCheck struct {
	err       error
	manifest  model.Manifest
	localPath string
	onVerify  func()
}

func (this *FakeIntegrityCheck) Verify(_ context.Context, manifest model.Manifest, localPath string) error {
	this.manifest = manifest
	this.localPath = localPath
	if this.onVerify != nil {
		this.onVerify()
	}
	return this.err
}
