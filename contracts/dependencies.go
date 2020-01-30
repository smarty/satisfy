package contracts

import (
	"errors"
	"fmt"
	"net/url"
)

type DependencyListing struct {
	Dependencies []Dependency `json:"dependencies"`
}

func (this *DependencyListing) Validate() error {
	inventory := make(map[string]struct{}) // map[PackageName+LocalDirectory]struct

	for _, dependency := range this.Dependencies {

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

		key := fmt.Sprintf("%s %s", dependency.PackageName, dependency.LocalDirectory)
		if _, found := inventory[key]; found {
			return errors.New("local directory conflict")
		}
		inventory[key] = struct{}{}
	}
	return nil
}

type Dependency struct {
	PackageName    string `json:"package_name"`
	PackageVersion string `json:"package_version"`
	RemoteAddress  URL    `json:"remote_address"`
	LocalDirectory string `json:"local_directory"`
}

func (this Dependency) ComposeRemoteAddress(fileName string) url.URL {
	return AppendRemotePath(
		url.URL(this.RemoteAddress),
		this.PackageName,
		this.PackageVersion,
		fileName,
	)
}

func (this Dependency) Title() string {
	return fmt.Sprintf("[%s @ %s]", this.PackageName, this.PackageVersion)
}
