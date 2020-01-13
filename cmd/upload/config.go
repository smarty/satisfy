package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/klauspost/compress/zstd"
	"github.com/smartystreets/gcs"
)

type Config struct {
	compressionAlgorithm string
	sourceDirectory      string
	packageName          string
	packageVersion       string
	remoteBucket         string
	remotePathPrefix     string
	googleCredentials    gcs.Credentials
}

func (this Config) composeRemotePath(extension string) string {
	return path.Join(this.remotePathPrefix, fmt.Sprintf("%s_%s.%s", this.packageName, this.packageVersion, extension))
}

func parseConfig() (config Config) {
	flag.StringVar(&config.compressionAlgorithm, "compression", "zstd", "The compression algorithm to use.")
	flag.StringVar(&config.sourceDirectory, "local", "", "The directory containing package data.")
	flag.StringVar(&config.packageName, "name", "", "The name of the package.")
	flag.StringVar(&config.packageVersion, "version", "", "The version of the package.")
	flag.StringVar(&config.remoteBucket, "remote-bucket", "", "The remote bucket name.")
	flag.StringVar(&config.remotePathPrefix, "remote-prefix", "", "The remote path prefix.")
	flag.Parse()

	raw, err := ioutil.ReadFile(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if err != nil {
		log.Fatal(err)
	}

	config.googleCredentials, err = gcs.ParseCredentialsFromJSON(raw)
	if err != nil {
		log.Fatal(err)
	}

	return config
}

var compression = map[string]func(io.Writer) io.WriteCloser {
	"zstd": func(writer io.Writer) io.WriteCloser {
		compressor, err := zstd.NewWriter(writer)
		if err != nil {
			log.Fatal(err)
		}
		return compressor
	},
	"gzip": func(writer io.Writer) io.WriteCloser {
		return gzip.NewWriter(writer)
	},
}