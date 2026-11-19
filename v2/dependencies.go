package satisfy

import (
	"fmt"
	"path/filepath"
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
		directory := filepath.Clean(dependency.LocalDirectory)
		key := dependency.PackageName + " " + directory
		if _, found := installed[key]; found {
			return fmt.Errorf("%w: package %q is installed into %q more than once",
				ErrInvalidConfig, dependency.PackageName, directory)
		}
		installed[key] = struct{}{}
	}
	return nil
}
