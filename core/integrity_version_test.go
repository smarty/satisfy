package core

import (
	"testing"

	"github.com/smartystreets/gunit"
)

func TestIntegrityVersionFixture(t *testing.T) {
    gunit.Run(new(IntegrityVersionFixture), t)
}

type IntegrityVersionFixture struct {
    *gunit.Fixture
}

func (this *IntegrityVersionFixture) Setup() {
}

func (this *IntegrityVersionFixture) Test() {
}
