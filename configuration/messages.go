package configuration

import (
	"encoding/json"

	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/logging"
)

const (
	StdInPath = "_STDIN_"
)

func EmitExampleDependenciesFile(logger *logging.Logger) {
	var listing contracts.DependencyListing
	listing.Listing = append(listing.Listing, contracts.Dependency{
		PackageName:    "example_package_name",
		PackageVersion: "0.0.1",
		RemoteAddress:  contracts.URL{Scheme: "gcs", Host: "bucket_name", Path: "/path/prefix"},
		LocalDirectory: "local/path",
	})

	raw, err := json.MarshalIndent(listing, "", "  ")
	if err != nil {
		logger.LogClean("%v", err)
	}

	logger.LogLineClean("Example json file: %s", string(raw))
}
