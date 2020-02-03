package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/url"
	"os"

	"github.com/smartystreets/gcs"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type PackageConfig struct {
	CompressionAlgorithm string         `json:"compression_algorithm"`
	CompressionLevel     int            `json:"compression_level"`
	SourceDirectory      string         `json:"source_directory"`
	PackageName          string         `json:"package_name"`
	PackageVersion       string         `json:"package_version"`
	RemoteAddressPrefix  *contracts.URL `json:"remote_address"`
}

type UploadConfig struct {
	MaxRetry          int
	GoogleCredentials gcs.Credentials
	JSONPath          string
	ForceUpload       bool
	PackageConfig     PackageConfig
}

func (this PackageConfig) ComposeRemoteAddress(filename string) url.URL {
	return contracts.AppendRemotePath(url.URL(*this.RemoteAddressPrefix), this.PackageName, this.PackageVersion, filename)
}

func parseUploadConfig(name string, args []string) (config UploadConfig) {
	flags := flag.NewFlagSet("satisfy "+name, flag.ExitOnError)
	flags.StringVar(&config.JSONPath,
		"json",
		"_STDIN_",
		"Path to file with config file or, if equal to _STDIN_, read from stdin.",
	)
	flags.IntVar(&config.MaxRetry,
		"max-retry",
		5,
		"HTTP max retry.",
	)
	flags.BoolVar(&config.ForceUpload,
		"force-upload",
		false,
		"When set, always upload package, even when it already exists at specified remote location.",
	)
	_ = flags.Parse(args)

	err := json.Unmarshal(readConfigFile(config), &config.PackageConfig)
	if err != nil {
		emitExamplePackageConfig()
		log.Fatal(err)
	}

	config.GoogleCredentials = ParseGoogleCredentialsFromEnvironment()

	return config
}

func readConfigFile(config UploadConfig) (raw []byte) {
	var err error
	if config.JSONPath == "_STDIN_" {
		raw, err = ioutil.ReadAll(os.Stdin)
	} else {
		raw, err = ioutil.ReadFile(config.JSONPath)
	}
	if err != nil {
		emitExamplePackageConfig()
		log.Fatal(err)
	}
	return raw
}

func emitExamplePackageConfig() {
	raw, _ := json.MarshalIndent(PackageConfig{
		CompressionAlgorithm: "zstd",
		CompressionLevel:     42,
		SourceDirectory:      "src/dir",
		PackageName:          "package-name",
		PackageVersion:       "0.0.1",
		RemoteAddressPrefix:  &contracts.URL{Scheme: "gcs", Host: "bucket_name", Path: "/path/prefix"},
	}, "", "  ")
	log.Println("Example JSON file:\n", string(raw))
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
