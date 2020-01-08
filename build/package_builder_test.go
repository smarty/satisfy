package build

import (
	"testing"

	"github.com/smartystreets/gunit"
)

func TestPackageBuilderFixture(t *testing.T) {
	gunit.Run(new(PackageBuilderFixture), t)
}

type PackageBuilderFixture struct {
	*gunit.Fixture
	builder *PackageBuilder
}

func (this *PackageBuilderFixture) Setup() {
}

func (this *PackageBuilderFixture) TestTarIsBuilt() {

}

/////////////////////////

type FakeArchiveWriter struct {
}

func (this *FakeArchiveWriter) Write([]byte) (int, error) {
	panic("implement me")
}

func (this *FakeArchiveWriter) Close() error {
	panic("implement me")
}

func (this *FakeArchiveWriter) WriteHeader(name string, size int64) {
	panic("implement me")
}
