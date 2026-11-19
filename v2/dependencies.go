package satisfy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateDependencies reports incomplete dependencies, and dependencies that
// would install the same package into the same local directory, with an error
// matching ErrInvalidConfig. Install calls it before any I/O.
func ValidateDependencies(dependencies ...Dependency) error {
	installed := make(map[string]struct{})
	for _, dependency := range dependencies {
		switch {
		case dependency.PackageName == "":
			return fmt.Errorf("%w: package name is required", ErrInvalidConfig)
		case dependency.PackageVersion == "":
			return fmt.Errorf("%w: package version is required (package %q)", ErrInvalidConfig, dependency.PackageName)
		case dependency.RemoteAddress.Host == "":
			return fmt.Errorf("%w: remote address is required (package %q)", ErrInvalidConfig, dependency.PackageName)
		case dependency.LocalDirectory == "":
			return fmt.Errorf("%w: local directory is required (package %q)", ErrInvalidConfig, dependency.PackageName)
		}
		directory := filepath.Clean(resolveLocalDirectory(dependency.LocalDirectory))
		key := dependency.PackageName + " " + directory
		if _, found := installed[key]; found {
			return fmt.Errorf("%w: package %q is installed into %q more than once",
				ErrInvalidConfig, dependency.PackageName, directory)
		}
		installed[key] = struct{}{}
	}
	return nil
}

// resolveLocalDirectories returns a copy of dependencies with a leading "~/",
// "$HOME" or "${HOME}" in each local directory replaced by the home directory.
func resolveLocalDirectories(dependencies []Dependency) []Dependency {
	resolved := make([]Dependency, 0, len(dependencies))
	for _, dependency := range dependencies {
		dependency.LocalDirectory = resolveLocalDirectory(dependency.LocalDirectory)
		resolved = append(resolved, dependency)
	}
	return resolved
}

func resolveLocalDirectory(value string) string {
	for _, prefix := range []string{"~/", "${HOME}", "$HOME"} {
		if rest, found := strings.CutPrefix(value, prefix); found {
			return filepath.Join(os.Getenv("HOME"), rest)
		}
	}
	return value
}
