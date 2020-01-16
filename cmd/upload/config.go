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
