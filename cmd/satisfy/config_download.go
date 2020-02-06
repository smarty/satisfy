package main

import (
	"encoding/json"
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

func parseDownloadConfig(args []string) (config DownloadConfig) {
	flags := flag.NewFlagSet("satisfy", flag.ExitOnError)
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

	err := flags.Parse(args)
	if err != nil {
		log.Fatal(err)
	}

	config.PackageFilter = flags.Args()

	config.GoogleCredentials = ParseGoogleCredentialsFromEnvironment()

	config.Dependencies = readDependencyListing(config.JSONPath)

	err = config.Dependencies.Validate()
	if err != nil {
		log.Fatal(err)
	}

	config.Dependencies.Listing = core.Filter(config.Dependencies.Listing, config.PackageFilter)

	if len(config.Dependencies.Listing) == 0 {
		log.Println("[WARN] No dependencies provided. You can go about your business. Move along.")
		emitExampleDependenciesFile()
	}

	return config
}

func ParseGoogleCredentialsFromEnvironment() gcs.Credentials {
	// FUTURE: support for ADC? (https://cloud.google.com/docs/authentication/production)
	path, found := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS")
	if !found {
		log.Fatal("Please set the GOOGLE_APPLICATION_CREDENTIALS environment variable.")
	}

	raw, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal("Could not open Google credentials file:", err)
	}

	credentials, err := gcs.ParseCredentialsFromJSON(raw)
	if err != nil {
		log.Fatal("Could not parse Google credentials file:", err)
	}

	return credentials
}

func readDependencyListing(path string) (listing contracts.DependencyListing) {
	if path == "_STDIN_" {
		return readFromReader(os.Stdin)
	} else {
		return readFromFile(path)
	}
}

func readFromFile(fileName string) (listing contracts.DependencyListing) {
	file, err := os.Open(fileName)
	if os.IsNotExist(err) {
		emitExampleDependenciesFile()
		log.Fatalln("Specified dependency file not found:", fileName)
	}
	if err != nil {
		log.Fatal(err)
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

func readFromReader(reader io.Reader) (listing contracts.DependencyListing) {
	decoder := json.NewDecoder(reader)
	err := decoder.Decode(&listing)
	if err != nil {
		log.Fatal(err)
	}
	return listing
}
