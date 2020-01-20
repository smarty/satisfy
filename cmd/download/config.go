package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/smartystreets/gcs"
)

type Config struct {
	Verify            bool
	GoogleCredentials gcs.Credentials
}

func parseConfig() (config Config) {
	flag.BoolVar(&config.Verify, "verify", false, "When set, perform file content validation on installed packages.")
	flag.Parse()
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
