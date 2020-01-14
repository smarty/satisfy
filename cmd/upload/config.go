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

const maxRetry = 5 // TODO: flag?

type Config struct {
	// TODO: compression level (& flag)
	compressionAlgorithm string
	sourceDirectory      string
	packageName          string
	packageVersion       string
	remoteBucket         string
	remotePathPrefix     string
	googleCredentials    gcs.Credentials
}

func (this Config) composeRemotePath(extension string) string {
	// TODO: directory for version containing 'manifest.json' and 'archive' (extension of archive supplied by manifest?)
	return path.Join(this.remotePathPrefix, this.packageName, fmt.Sprintf("%s.%s", this.packageVersion, extension))
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

var compression = map[string]func(io.Writer) io.WriteCloser{
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
