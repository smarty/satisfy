package main

import (
	"flag"

	"bitbucket.org/smartystreets/satisfy/cmd"
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

	config.GoogleCredentials = cmd.ParseGoogleCredentialsFromEnvironment()

	return config
}
