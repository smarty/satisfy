package model

import (
	"slices"
	"strings"
)

// MergeTags points each named tag at the supplied version while preserving
// unrelated existing tags. The result is sorted by tag name.
func MergeTags(existing []Tag, names []string, version string) []Tag {
	merged := append([]Tag(nil), existing...)
	for _, name := range names {
		merged = upsertTag(merged, Tag{Name: name, Version: version})
	}
	sortTags(merged)
	return merged
}

// ApplyTagModifications adds or updates each tag in add, then removes each tag
// named in remove (a no-op for names not present). The result is sorted by tag name.
func ApplyTagModifications(existing, add, remove []Tag) []Tag {
	merged := append([]Tag(nil), existing...)
	for _, tag := range add {
		merged = upsertTag(merged, tag)
	}
	for _, tag := range remove {
		merged = removeTag(merged, tag.Name)
	}
	sortTags(merged)
	return merged
}

func upsertTag(tags []Tag, tag Tag) []Tag {
	for i := range tags {
		if tags[i].Name == tag.Name {
			tags[i].Version = tag.Version
			return tags
		}
	}
	return append(tags, tag)
}

func removeTag(tags []Tag, name string) []Tag {
	for i := range tags {
		if tags[i].Name == name {
			return append(tags[:i], tags[i+1:]...)
		}
	}
	return tags
}

func sortTags(tags []Tag) {
	slices.SortFunc(tags, func(a, b Tag) int { return strings.Compare(a.Name, b.Name) })
}
