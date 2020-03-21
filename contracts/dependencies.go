package contracts

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type DependencyListing struct {
	Listing []Dependency `json:"dependencies"`
}

func (this *DependencyListing) Validate() error {
	inventory := make(map[string]struct{}) // map[PackageName+LocalDirectory]struct

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
func formatLocalDirectory(value string) string {
	return filepath.Join(os.Getenv("HOME"), value)
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
func (this Dependency) ComposeLatestManifestRemoteAddress() url.URL {
	address := url.URL(this.RemoteAddress)
	address.Path = path.Join(strings.TrimPrefix(address.Path, "/"), this.PackageName, RemoteManifestFilename)
	return address
}
func (this Dependency) Title() string {
	return fmt.Sprintf("[%s @ %s]", this.PackageName, this.PackageVersion)
}

func (this Dependency) ComposeRemoteManifestAddress() url.URL {
	if this.PackageVersion == "latest" {
		return this.ComposeLatestManifestRemoteAddress()
	} else {
		return this.ComposeRemoteAddress(RemoteManifestFilename)
	}
}
