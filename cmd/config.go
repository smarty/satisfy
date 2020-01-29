package cmd

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

type Config struct {
	CompressionAlgorithm string          `json:"compression_algorithm"`
	CompressionLevel     int             `json:"compression_level"`
	SourceDirectory      string          `json:"source_directory"`
	PackageName          string          `json:"package_name"`
	PackageVersion       string          `json:"package_version"`
	RemoteAddressPrefix  URL             `json:"remote_address"`
	MaxRetry             int             `json:"-"`
	GoogleCredentials    gcs.Credentials `json:"-"`
	JSONPath             string          `json:"-"`
	ForceUpload          bool            `json:"-"`
}

func (this Config) ComposeRemoteAddress(filename string) url.URL {
	return contracts.AppendRemotePath(url.URL(this.RemoteAddressPrefix), this.PackageName, this.PackageVersion, filename)
}

const (
	RemoteManifestFilename = "manifest.json"
	RemoteArchiveFilename  = "archive"
)

func ParseConfig(name string, args []string) (config Config) {
	flags := flag.NewFlagSet("satisfy "+name, flag.ExitOnError)
	flags.StringVar(&config.JSONPath, "json", "upload.json", "The path to the JSON config file.")
	flags.IntVar(&config.MaxRetry, "max-retry", 5, "HTTP max retry.")
	flags.BoolVar(&config.ForceUpload, "force-upload", false,
		"When set, always upload package, even when it already exists at specified remote location.")
	_ = flags.Parse(args)

	raw, err := ioutil.ReadFile(config.JSONPath)
	if err != nil {
		log.Fatal(err) // TODO: emit sample json file
	}

	err = json.Unmarshal(raw, &config)
	if err != nil {
		log.Fatal(err) // TODO: emit sample json file
	}

	config.GoogleCredentials = ParseGoogleCredentialsFromEnvironment()

	return config
}

func ParseGoogleCredentialsFromEnvironment() gcs.Credentials {
	// TODO: support for ADC? (https://cloud.google.com/docs/authentication/production)
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
