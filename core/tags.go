package core

import (
	"sort"

	"github.com/smarty/satisfy/contracts"
)

// MergeTags points each named tag at the supplied version while preserving
// unrelated existing tags. The result is sorted by tag name.
func MergeTags(existing []contracts.Tag, names []string, version string) []contracts.Tag {
	merged := append([]contracts.Tag(nil), existing...)
	for _, name := range names {
		merged = upsertTag(merged, contracts.Tag{Name: name, Version: version})
	}
	sortTags(merged)
	return merged
}

// ApplyTagModifications adds or updates each tag in add, then removes each tag
// named in remove (a no-op for names not present). The result is sorted by tag name.
func ApplyTagModifications(existing, add, remove []contracts.Tag) []contracts.Tag {
	merged := append([]contracts.Tag(nil), existing...)
	for _, tag := range add {
		merged = upsertTag(merged, tag)
	}
	for _, tag := range remove {
		merged = removeTag(merged, tag.Name)
	}
	sortTags(merged)
	return merged
}

func upsertTag(tags []contracts.Tag, tag contracts.Tag) []contracts.Tag {
	for i := range tags {
		if tags[i].Name == tag.Name {
			tags[i].Version = tag.Version
			return tags
		}
	}
	return append(tags, tag)
}

func removeTag(tags []contracts.Tag, name string) []contracts.Tag {
	for i := range tags {
		if tags[i].Name == name {
			return append(tags[:i], tags[i+1:]...)
		}
	}
	return tags
}

func sortTags(tags []contracts.Tag) {
	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
}
