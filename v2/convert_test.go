package satisfy

import (
	"reflect"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2/internal/model"
)

func TestConvertFixture(t *testing.T) {
	gunit.Run(new(ConvertFixture), t)
}

// ConvertFixture guards the copies between the public types and their
// internal/model counterparts: a field added to one side but not the other,
// or not copied, fails here.
type ConvertFixture struct {
	*gunit.Fixture
}

var address = URL{Scheme: "gcs", Host: "bucket", Path: "/prefix"}

func (this *ConvertFixture) assertSameFields(public, internal any) {
	publicType, internalType := reflect.TypeOf(public), reflect.TypeOf(internal)
	var publicFields, internalFields []string
	for field := range publicType.Fields() {
		publicFields = append(publicFields, field.Name+" "+field.Tag.Get("json"))
	}
	for field := range internalType.Fields() {
		internalFields = append(internalFields, field.Name+" "+field.Tag.Get("json"))
	}
	this.So(publicFields, should.Equal, internalFields)
}

// assertNoZeroFields fails when any field of value (a struct) is zero, which
// means the conversion that produced it skipped a field.
func (this *ConvertFixture) assertNoZeroFields(value any) {
	reflected := reflect.ValueOf(value)
	for index := range reflected.NumField() {
		this.So(reflected.Field(index).IsZero(), should.BeFalse)
	}
}

func (this *ConvertFixture) TestPublicTypesMirrorInternalTypes() {
	this.assertSameFields(PackageRef{}, model.PackageRef{})
	this.assertSameFields(Dependency{}, model.Dependency{})
	this.assertSameFields(PackageConfig{}, model.PackageConfig{})
	this.assertSameFields(TagModification{}, model.TagModificationConfig{})
	this.assertSameFields(Tag{}, model.Tag{})
	this.assertSameFields(Manifest{}, model.Manifest{})
	this.assertSameFields(Archive{}, model.Archive{})
	this.assertSameFields(ArchiveItem{}, model.ArchiveItem{})
	this.assertSameFields(Progress{}, model.Progress{})
}

func (this *ConvertFixture) TestPackageRef() {
	converted := PackageRef{RemoteAddress: address, Name: "/name/"}.internal()
	this.So(converted, should.Equal, model.PackageRef{RemoteAddress: model.URL(address), Name: "name"})
}

func (this *ConvertFixture) TestDependency() {
	converted := Dependency{
		PackageName: "name", PackageVersion: "1.2.3", RemoteAddress: address, LocalDirectory: "local",
	}.internal()
	this.assertNoZeroFields(converted)
}

func (this *ConvertFixture) TestPackageConfig() {
	converted := PackageConfig{
		CompressionAlgorithm: "zstd", CompressionLevel: 3,
		SourceDirectory: "directory", SourceFile: "file", SourcePath: "path",
		PackageName: "name", PackageVersion: "1.2.3", RemoteAddress: address, Tags: []string{"stable"},
	}.internal()
	this.assertNoZeroFields(converted)
}

func (this *ConvertFixture) TestTagModification() {
	converted := TagModification{
		PackageName: "name", RemoteAddress: address,
		Add: []Tag{{Name: "a", Version: "1"}}, Delete: []Tag{{Name: "b"}},
	}.internal()
	this.assertNoZeroFields(converted)
	this.So(converted.Add, should.Equal, []model.Tag{{Name: "a", Version: "1"}})
	this.So(converted.Delete, should.Equal, []model.Tag{{Name: "b"}})
	this.So(TagModification{}.internal().Add, should.BeNil)
}

func (this *ConvertFixture) TestManifest() {
	converted := publicManifest(model.Manifest{
		Name:    "name",
		Version: "1.2.3",
		Archive: model.Archive{
			Filename: "archive", Size: 42, MD5Checksum: []byte{1}, CompressionAlgorithm: "zstd",
			Contents: []model.ArchiveItem{{Path: "file", Size: 7, MD5Checksum: []byte{2}}},
		},
		Tags: []model.Tag{{Name: "stable", Version: "1.2.3"}},
	})
	this.assertNoZeroFields(converted)
	this.assertNoZeroFields(converted.Archive)
	this.So(converted.Archive.Contents, should.Equal, []ArchiveItem{{Path: "file", Size: 7, MD5Checksum: []byte{2}}})
	this.So(converted.Tags, should.Equal, []Tag{{Name: "stable", Version: "1.2.3"}})
	this.So(publicManifest(model.Manifest{}).Tags, should.BeNil)
}

func (this *ConvertFixture) TestProgress() {
	converted := publicProgress(model.Progress{
		Phase: model.ProgressExtracting, Name: "file", Written: 1, Total: 2, Done: true,
	})
	this.assertNoZeroFields(converted)
	this.So(converted.Phase, should.Equal, ProgressExtracting)
}

func (this *ConvertFixture) TestProgressPhasesMatch() {
	this.So(ProgressArchiving, should.Equal, ProgressPhase(model.ProgressArchiving))
	this.So(ProgressDownloading, should.Equal, ProgressPhase(model.ProgressDownloading))
	this.So(ProgressExtracting, should.Equal, ProgressPhase(model.ProgressExtracting))
}

func (this *ConvertFixture) TestURLMarshalsLikeTheInternalURL() {
	raw, err := address.MarshalJSON()
	this.So(err, should.BeNil)
	this.So(string(raw), should.Equal, `"gcs://bucket/prefix"`)

	var decoded URL
	this.So(decoded.UnmarshalJSON(raw), should.BeNil)
	this.So(decoded, should.Equal, address)
	this.So(decoded.String(), should.Equal, "gcs://bucket/prefix")
}
