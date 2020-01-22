package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/smartystreets/gcs"
)

type Config struct {
	MaxRetry          int
	Verify            bool
	GoogleCredentials gcs.Credentials
	jsonPath          string
}

func parseConfig() (config Config) {
	flag.IntVar(&config.MaxRetry,
		"max-retry",
		5,
		"How many times to retry attempts to download packages.",
	)
	flag.BoolVar(&config.Verify,
		"verify",
		false,
		"When set, perform file content validation on installed packages.",
	)
	flag.StringVar(&config.jsonPath,
		"json",
		"_STDIN_",
		"Path to file with dependency listing or, if equal to _STDIN_, read from stdin.",
	)
	flag.Parse()

	googleCredentialsPath, found := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS")
	if !found {
		log.Fatal("Please set the GOOGLE_APPLICATION_CREDENTIALS environment variable.")
	}
	raw, err := ioutil.ReadFile(googleCredentialsPath)
	if err != nil {
		log.Fatal("Could not open google credentials file:", err)
	}

	config.GoogleCredentials, err = gcs.ParseCredentialsFromJSON(raw)
	if err != nil {
		log.Fatal(err)
	}

	return config
}
