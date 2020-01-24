package cmd

import (
	"errors"
	"fmt"
	"net/url"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type DependencyListing struct {
	Dependencies []Dependency `json:"dependencies"`
}

func (this *DependencyListing) Validate() error {
	// TODO: is there a risk of collisions on the downloaded manifest name? for example if two packages are named "data"
	// and they're both downloaded from different locations to the same directory.

	inventory := make(map[string]string)

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
		if version, found := inventory[key]; found && version != dependency.PackageVersion {
			return errors.New("local directory conflict")
		}
		inventory[key] = dependency.PackageVersion
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
	return contracts.AppendRemotePath(
		url.URL(this.RemoteAddress),
		this.PackageName,
		this.PackageVersion,
		fileName,
	)
}

func (this Dependency) Title() string {
	return fmt.Sprintf("[%s @ %s]", this.PackageName, this.PackageVersion)
}
