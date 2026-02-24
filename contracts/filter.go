package contracts

import (
	"slices"
)

func Filter(original []Dependency, filter []string) (filtered []Dependency) {
	if len(filter) == 0 {
		return original
	}

	for _, dependency := range original {
		if slices.Contains(filter, dependency.PackageName) {
			filtered = append(filtered, dependency)
		}
	}

	return filtered
}
