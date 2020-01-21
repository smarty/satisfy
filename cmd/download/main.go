package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"bitbucket.org/smartystreets/satisfy/cmd"
	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/core"
	"bitbucket.org/smartystreets/satisfy/shell"
)

func main() {
	config := parseConfig()

	decoder := json.NewDecoder(os.Stdin)
	var listing cmd.DependencyListing
	err := decoder.Decode(&listing)
	if err != nil {
		log.Fatal(err)
	}

	err = listing.Validate()
	if err != nil {
		log.Fatal(err)
	}

	working, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	disk := shell.NewDiskFileSystem(working)
	client := shell.NewGoogleCloudStorageClient(cmd.NewHTTPClient(), config.GoogleCredentials, http.StatusOK)
	installer := core.NewPackageInstaller(client, disk)
	integrity := core.NewCompoundIntegrityCheck(
		core.NewFileListingIntegrityChecker(disk),
		core.NewFileContentIntegrityCheck(md5.New(), disk, config.Verify),
	)

	for _, dependency := range listing.Dependencies { // TODO Concurrent installation
		manifest, err := loadManifest(dependency)

		if err == errNotInstalled || manifest.Version != dependency.Version || integrity.Verify(manifest) != nil {
			manifest, err = installer.InstallManifest(contracts.InstallationRequest{
				RemoteAddress: *dependency.RemoteAddress.Value(), // TODO Combine with manifest path
				LocalPath:     dependency.LocalDirectory,
			})
			if err != nil {
				log.Fatal(err) // TODO Don't prevent other packages from installing
			}

			err = installer.InstallPackage(manifest, contracts.InstallationRequest{
				RemoteAddress: *dependency.RemoteAddress.Value(), // TODO Combine with archive path
				LocalPath:     dependency.LocalDirectory,
			})
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func loadManifest(dependency cmd.Dependency) (manifest contracts.Manifest, err error) {
	path := core.ComposeManifestPath(dependency.LocalDirectory, dependency.Name)

	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return manifest, errNotInstalled
	}

	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return manifest, err
	}

	err = json.Unmarshal(raw, &manifest)
	if err != nil {
		return manifest, err
	}

	return manifest, nil
}

var (
	errNotInstalled = errors.New("package not yet installed")
)
