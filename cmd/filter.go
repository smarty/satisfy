package cmd

func Filter(original []Dependency, filter []string) (filtered []Dependency) {
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
