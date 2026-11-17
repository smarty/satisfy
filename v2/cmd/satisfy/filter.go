package main

import (
	"slices"

	"github.com/smarty/satisfy/v2/internal/model"
)

func Filter(original []model.Dependency, filter []string) (filtered []model.Dependency) {
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
