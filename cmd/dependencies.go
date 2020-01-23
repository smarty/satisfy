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
	inventory := make(map[string]string)

	for _, dependency := range this.Dependencies {

		if dependency.LocalDirectory == "" {
			return errors.New("local directory is required")
		}
		if dependency.Name == "" {
			return errors.New("name is required")
		}
		if dependency.Version == "" {
			return errors.New("version is required")
		}
		if dependency.RemoteAddress.Value().String() == "" {
			return errors.New("remote address is required")
		}

		key := fmt.Sprintf("%s %s", dependency.Name, dependency.LocalDirectory)
		if version, found := inventory[key]; found && version != dependency.Version {
			return errors.New("local directory conflict")
		}
		inventory[key] = dependency.Version
	}
	return nil
}

type Dependency struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	RemoteAddress  URL    `json:"remote_address"`
	LocalDirectory string `json:"local_directory"`
}

func (this Dependency) ComposeRemoteAddress(fileName string) url.URL {
	return contracts.AppendRemotePath(
		url.URL(this.RemoteAddress),
		this.Name,
		this.Version,
		fileName,
	)
}

func (this Dependency) Title() string {
	return fmt.Sprintf("%s@%s", this.Name, this.Version)
}
