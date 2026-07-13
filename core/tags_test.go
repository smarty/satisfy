package core

import (
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/contracts"
)

func TestTagsFixture(t *testing.T) {
	gunit.Run(new(TagsFixture), t)
}

type TagsFixture struct {
	*gunit.Fixture
	existing []contracts.Tag
}

func (this *TagsFixture) Setup() {
	this.existing = []contracts.Tag{
		{Name: "stable", Version: "2026.01.A"},
		{Name: "experimental", Version: "2026.01.B"},
	}
}

func (this *TagsFixture) TestMergeTagsUpdatesExistingAndAppendsNew() {
	merged := MergeTags(this.existing, []string{"experimental", "release"}, "2026.02.A")

	this.So(merged, should.Resemble, []contracts.Tag{
		{Name: "experimental", Version: "2026.02.A"},
		{Name: "release", Version: "2026.02.A"},
		{Name: "stable", Version: "2026.01.A"},
	})
}

func (this *TagsFixture) TestMergeTagsWithNoNamesPreservesExisting() {
	merged := MergeTags(this.existing, nil, "2026.02.A")

	this.So(merged, should.Resemble, []contracts.Tag{
		{Name: "experimental", Version: "2026.01.B"},
		{Name: "stable", Version: "2026.01.A"},
	})
}

func (this *TagsFixture) TestMergeTagsWithNothingAtAll() {
	this.So(MergeTags(nil, nil, "2026.02.A"), should.BeEmpty)
}

func (this *TagsFixture) TestMergeTagsDoesNotMutateExisting() {
	_ = MergeTags(this.existing, []string{"stable"}, "2026.02.A")

	this.So(this.existing[0], should.Resemble, contracts.Tag{Name: "stable", Version: "2026.01.A"})
}

func (this *TagsFixture) TestApplyTagModifications() {
	applied := ApplyTagModifications(this.existing,
		[]contracts.Tag{
			{Name: "stable", Version: "2026.01.B"},
			{Name: "marks-favorite", Version: "2026.01.B"},
		},
		[]contracts.Tag{
			{Name: "experimental"},
			{Name: "never-existed"},
		},
	)

	this.So(applied, should.Resemble, []contracts.Tag{
		{Name: "marks-favorite", Version: "2026.01.B"},
		{Name: "stable", Version: "2026.01.B"},
	})
}

func (this *TagsFixture) TestApplyTagModificationsCanDeleteEveryTag() {
	applied := ApplyTagModifications(this.existing, nil,
		[]contracts.Tag{{Name: "stable"}, {Name: "experimental"}})

	this.So(applied, should.BeEmpty)
}
