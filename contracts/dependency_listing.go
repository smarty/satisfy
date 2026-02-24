package contracts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DependencyListing struct {
	Credentials string       `json:"credentials"`
	Listing     []Dependency `json:"dependencies"`
}

func (this *DependencyListing) Validate() error {
	inventory := make(map[string]struct{})

	for i, dependency := range this.Listing {
		if dependency.LocalDirectory == "" {
			return errors.New("local directory is required")
		}

		if dependency.PackageName == "" {
			return errors.New("name is required")
		}

		if dependency.PackageVersion == "" {
			return errors.New("version is required")
		}

		if dependency.RemoteAddress.Value().String() == "" {
			return errors.New("remote address is required")
		}

		dependency.LocalDirectory = resolveLocalDirectory(dependency.LocalDirectory)
		this.Listing[i] = dependency
		key := fmt.Sprintf("%s %s", dependency.PackageName, dependency.LocalDirectory)
		if _, found := inventory[key]; found {
			return errors.New("local directory conflict")
		}

		inventory[key] = struct{}{}
	}

	return nil
}

func formatLocalDirectory(value string) string {
	return filepath.Join(os.Getenv("HOME"), value)
}

func resolveLocalDirectory(value string) string {
	if strings.HasPrefix(value, "~/") {
		return formatLocalDirectory(value[2:])
	}

	if strings.HasPrefix(value, "$HOME") {
		return formatLocalDirectory(value[5:])
	}

	if strings.HasPrefix(value, "${HOME}") {
		return formatLocalDirectory(value[7:])
	}

	return value
}
