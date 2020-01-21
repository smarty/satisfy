package cmd

import (
	"net/url"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type DependencyListing struct {
	Dependencies []Dependency `json:"dependencies"`
}

func (this *DependencyListing) Validate() error {
	return nil // TODO
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