package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/klauspost/compress/zstd"
	"github.com/smartystreets/gcs"
)

type Config struct {
	CompressionAlgorithm string          `json:"compression_algorithm"`
	CompressionLevel     int             `json:"compression_level"`
	SourceDirectory      string          `json:"source_directory"`
	PackageName          string          `json:"package_name"`
	PackageVersion       string          `json:"package_version"`
	RemoteBucket         string          `json:"remote_bucket"`
	RemotePathPrefix     string          `json:"remote_path_prefix"`
	MaxRetry             int             `json:"max_retry"`
	ForceUpload          bool            `json:"force_upload"`
	GoogleCredentials    gcs.Credentials `json:"-"`
	JSONPath             string          `json:"-"`
}

func (this Config) composeRemotePath(filename string) string {
	return path.Join(this.RemotePathPrefix, this.PackageName, this.PackageVersion, filename)
}

const (
	remoteManifestFilename = "manifest.json"
	remoteArchiveFilename  = "archive"
)

func parseConfig() (config Config) {
	flag.StringVar(&config.JSONPath, "json", "",
		"If provided, the JSON file with config values. "+
			"Any provided JSON values will overwrite the corresponding command line values")
	flag.StringVar(&config.CompressionAlgorithm, "compression", "zstd",
		"The compression algorithm to use. The only two valid values are zstd and gzip.")
	flag.IntVar(&config.CompressionLevel, "compression-level", 5,
		"The compression level to use. See the documentation corresponding to the specified compression flag value.")
	flag.StringVar(&config.SourceDirectory, "local", "", "The directory containing package data.")
	flag.StringVar(&config.PackageName, "name", "", "The name of the package.")
	flag.StringVar(&config.PackageVersion, "version", "", "The version of the package.")
	flag.StringVar(&config.RemoteBucket, "remote-bucket", "", "The remote bucket name.")
	flag.StringVar(&config.RemotePathPrefix, "remote-prefix", "", "The remote path prefix.")
	flag.IntVar(&config.MaxRetry, "max-retry", 5, "The max retry value.")
	flag.BoolVar(&config.ForceUpload, "force-upload", false,
		"When set, build and upload the package even if it already exists remotely.")
	flag.Parse()
	if config.JSONPath != "" {
		raw, err := ioutil.ReadFile("config.json")
		if err != nil {
			log.Fatal(err)
		}
		err = json.Unmarshal(raw, &config)
		if err != nil {
			log.Fatal(err)
		}
	}
	raw, err := ioutil.ReadFile(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if err != nil {
		log.Fatal(err)
	}

	config.GoogleCredentials, err = gcs.ParseCredentialsFromJSON(raw)
	if err != nil {
		log.Fatal(err)
	}

	return config
}

var compression = map[string]func(_ io.Writer, level int) io.WriteCloser{
	"zstd": func(writer io.Writer, level int) io.WriteCloser {
		compressor, err := zstd.NewWriter(writer, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)))
		if err != nil {
			log.Fatal(err)
		}
		return compressor
	},
	"gzip": func(writer io.Writer, level int) io.WriteCloser {
		compressor, err := gzip.NewWriterLevel(writer, level)
		if err != nil {
			log.Panicln(err)
		}
		return compressor
	},
}
