package core

import "github.com/smartystreets/satisfy/contracts"

func Filter(original []contracts.Dependency, filter []string) (filtered []contracts.Dependency) {
	if len(filter) == 0 {
		return original
	}
	for _, dependency := range original {
		if contains(filter, dependency.PackageName) {
			filtered = append(filtered, dependency)
		}
	}
	return filtered
}

func contains(haystack []string, needle string) bool {
	for _, straw := range haystack {
		if straw == needle {
			return true
		}
	}
	return false
}
