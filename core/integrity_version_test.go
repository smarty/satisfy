package core

import (
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

func TestIntegrityVersionFixture(t *testing.T) {
	gunit.Run(new(IntegrityVersionFixture), t)
}

type IntegrityVersionFixture struct {
	*gunit.Fixture

	checker *VersionIntegrityCheck
}

func (this *IntegrityVersionFixture) Setup() {
	this.checker = NewVersionIntegrityCheck("1.2.3")
}

func (this *IntegrityVersionFixture) TestCorrectVersion() {
	manifest := contracts.Manifest{Version: "1.2.3"}

	this.So(this.checker.Verify(manifest), should.BeNil)
}

func (this *IntegrityVersionFixture) TestIncorrectVersion() {
	manifest := contracts.Manifest{Version: "1.2.4"}

	this.So(this.checker.Verify(manifest), should.Equal, errVersionMismatch)
}
