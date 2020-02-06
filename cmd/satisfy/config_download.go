package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"bitbucket.org/smartystreets/satisfy/core"
	"github.com/smartystreets/gcs"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type DownloadConfig struct {
	MaxRetry          int
	QuickVerification bool
	JSONPath          string
	GoogleCredentials gcs.Credentials
	PackageFilter     []string
	Dependencies      contracts.DependencyListing
}

func parseDownloadConfig(args []string) (config DownloadConfig, err error) {
	flags := flag.NewFlagSet("satisfy", flag.ContinueOnError)
	flags.IntVar(&config.MaxRetry,
		"max-retry",
		5,
		"How many times to retry attempts to download packages.",
	)
	flags.BoolVar(&config.QuickVerification,
		"quick",
		true,
		"When set to false, perform full file content validation on installed packages.",
	)
	flags.StringVar(&config.JSONPath,
		"json",
		"_STDIN_",
		"Path to file with dependency listing or, if equal to _STDIN_, read from stdin.",
	)

	flags.Usage = func() {
		output := flags.Output()
		_, _ = fmt.Fprintf(output, "Usage of %s:\n", os.Args[0])
		flags.PrintDefaults()
		_, _ = fmt.Fprintln(output)
		_, _ = fmt.Fprintln(output, "  Package names may be passed as non-flag arguments and will serve as a filter "+
			"against the provided dependency listing.")
		_, _ = fmt.Fprintln(output)
		_, _ = fmt.Fprintln(output, "  The satisfy tool also provides 2 additional subcommands:")
		_, _ = fmt.Fprintln(output, "	check	Has package@version already been uploaded according to json config?")
		_, _ = fmt.Fprintln(output, "	upload	Upload package contents according to json config.")
		_, _ = fmt.Fprintln(output)
	}

	err = flags.Parse(args)
	if err != nil {
		return DownloadConfig{}, err
	}

	config.GoogleCredentials, err = parseGoogleCredentialsFromEnvironment()
	if err != nil {
		return DownloadConfig{}, err
	}

	config.Dependencies, err = loadDependencyListing(config.JSONPath, flags.Args())
	if err != nil {
		return DownloadConfig{}, err
	}

	return config, nil
}

func loadDependencyListing(path string, filter []string) (contracts.DependencyListing, error) {
	dependencies, err := readDependencyListing(path)
	if err != nil {
		return contracts.DependencyListing{}, err
	}

	err = dependencies.Validate()
	if err != nil {
		return contracts.DependencyListing{}, err
	}

	dependencies.Listing = core.Filter(dependencies.Listing, filter)

	if len(dependencies.Listing) == 0 {
		log.Println("[WARN] No dependencies provided. You can go about your business. Move along.")
		emitExampleDependenciesFile()
	}
	return dependencies, nil
}

func parseGoogleCredentialsFromEnvironment() (gcs.Credentials, error) {
	// FUTURE: support for ADC? (https://cloud.google.com/docs/authentication/production)
	path, found := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS")
	if !found {
		return gcs.Credentials{}, errors.New("the GOOGLE_APPLICATION_CREDENTIALS environment variable is required")
	}

	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return gcs.Credentials{}, fmt.Errorf("could not open Google credentials file: %w", err)
	}

	credentials, err := gcs.ParseCredentialsFromJSON(raw)
	if err != nil {
		return gcs.Credentials{}, fmt.Errorf("could not parse Google credentials file: %w", err)
	}

	return credentials, nil
}

func readDependencyListing(path string) (contracts.DependencyListing, error) {
	if path == "_STDIN_" {
		return readFromReader(os.Stdin)
	} else {
		return readFromFile(path)
	}
}

func readFromFile(fileName string) (listing contracts.DependencyListing, err error) {
	file, err := os.Open(fileName)
	if os.IsNotExist(err) {
		emitExampleDependenciesFile()
		return listing, fmt.Errorf("specified dependency file (%q) not found: %w", fileName, err)
	}
	if err != nil {
		return listing, fmt.Errorf("could not open specified dependency file (%q): %w", fileName, err)
	}
	defer func() { _ = file.Close() }()
	return readFromReader(file)
}

func emitExampleDependenciesFile() {
	var listing contracts.DependencyListing
	listing.Listing = append(listing.Listing, contracts.Dependency{
		PackageName:    "example_package_name",
		PackageVersion: "0.0.1",
		RemoteAddress:  contracts.URL{Scheme: "gcs", Host: "bucket_name", Path: "/path/prefix"},
		LocalDirectory: "local/path",
	})
	raw, err := json.MarshalIndent(listing, "", "  ")
	if err != nil {
		log.Print(err)
	}
	log.Print("Example json file:\n", string(raw))
}

func readFromReader(reader io.Reader) (listing contracts.DependencyListing, err error) {
	decoder := json.NewDecoder(reader)
	err = decoder.Decode(&listing)
	return listing, err
}
