package core

import (
	"slices"

	"github.com/smarty/satisfy/contracts"
)

func Filter(original []contracts.Dependency, filter []string) (filtered []contracts.Dependency) {
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
