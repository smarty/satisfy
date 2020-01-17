package cmd

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/smartystreets/gcs"
)

type Config struct {
	CompressionAlgorithm string          `json:"compression_algorithm"`
	CompressionLevel     int             `json:"compression_level"`
	SourceDirectory      string          `json:"source_directory"`
	PackageName          string          `json:"package_name"`
	PackageVersion       string          `json:"package_version"`
	RemoteBucket         string          `json:"remote_bucket"` // TODO: Combine with RemotePathPrefix (RemoteAddress)
	RemotePathPrefix     string          `json:"remote_path_prefix"`
	MaxRetry             int             `json:"max_retry"`
	GoogleCredentials    gcs.Credentials `json:"-"`
	JSONPath             string          `json:"-"`
}

func (this Config) ComposeRemotePath(filename string) string {
	return path.Join(this.RemotePathPrefix, this.PackageName, this.PackageVersion, filename)
}

const (
	RemoteManifestFilename = "manifest.json"
	RemoteArchiveFilename  = "archive"
)

func ParseConfig() (config Config) {
	flag.StringVar(&config.JSONPath, "json", "config.json", "The path to the JSON config file.")
	flag.Parse()

	raw, err := ioutil.ReadFile(config.JSONPath)
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(raw, &config)
	if err != nil {
		log.Fatal(err)
	}

	raw, err = ioutil.ReadFile(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if err != nil {
		log.Fatal(err)
	}

	config.GoogleCredentials, err = gcs.ParseCredentialsFromJSON(raw)
	if err != nil {
		log.Fatal(err)
	}

	return config
}
