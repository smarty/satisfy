package main

import (
	"compress/gzip"
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
	compressionAlgorithm string
	compressionLevel     int
	sourceDirectory      string
	packageName          string
	packageVersion       string
	remoteBucket         string
	remotePathPrefix     string
	googleCredentials    gcs.Credentials
	maxRetry             int
}

func (this Config) composeRemotePath(filename string) string {
	return path.Join(this.remotePathPrefix, this.packageName, this.packageVersion, filename)
}

const (
	remoteManifestFilename = "manifest.json"
	remoteArchiveFilename  = "archive"
)

func parseConfig() (config Config) {
	flag.StringVar(&config.compressionAlgorithm, "compression", "zstd",
		"The compression algorithm to use. The only two valid values are zstd and gzip.")
	flag.IntVar(&config.compressionLevel, "compression-level", 5,
		"The compression level to use. See the documentation corresponding to the specified compression flag value.")
	flag.StringVar(&config.sourceDirectory, "local", "", "The directory containing package data.")
	flag.StringVar(&config.packageName, "name", "", "The name of the package.")
	flag.StringVar(&config.packageVersion, "version", "", "The version of the package.")
	flag.StringVar(&config.remoteBucket, "remote-bucket", "", "The remote bucket name.")
	flag.StringVar(&config.remotePathPrefix, "remote-prefix", "", "The remote path prefix.")
	flag.IntVar(&config.maxRetry, "max-retry", 5, "The max retry value.")
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
