package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/smartystreets/gcs"
)

type DownloadConfig struct {
	MaxRetry          int
	QuickVerification bool
	JSONPath          string
	GoogleCredentials gcs.Credentials
	packageFilter     []string
}

func parseDownloadConfig(args []string) (config DownloadConfig) {
	flags := flag.NewFlagSet("satisfy", flag.ExitOnError)
	flags.IntVar(&config.MaxRetry,
		"max-retry",
		5,
		"How many times to retry attempts to download packages.",
	)
	flags.BoolVar(&config.QuickVerification,
		"quick",
		true,
		"When set to false, perform full file content validation on installed packages.",
	)
	flags.StringVar(&config.JSONPath,
		"json",
		"_STDIN_",
		"Path to file with dependency listing or, if equal to _STDIN_, read from stdin.",
	)

	flags.Usage = func() {
		output := flags.Output()
		_, _ = fmt.Fprintf(output, "Usage of %s:\n", os.Args[0])
		flags.PrintDefaults()
		_, _ = fmt.Fprintln(output)
		_, _ = fmt.Fprintln(output, "  Package names may be passed as non-flag arguments and will serve as a filter "+
			"against the provided dependency listing.")
		_, _ = fmt.Fprintln(output)
		_, _ = fmt.Fprintln(output, "  The satisfy tool also provides 2 additional subcommands:")
		_, _ = fmt.Fprintln(output, "	check	Has package@version already been uploaded according to json config?")
		_, _ = fmt.Fprintln(output, "	upload	Upload package contents according to json config.")
		_, _ = fmt.Fprintln(output)
	}

	err := flags.Parse(args)
	if err != nil {
		log.Fatal(err)
	}

	config.packageFilter = flags.Args()

	config.GoogleCredentials = ParseGoogleCredentialsFromEnvironment()

	return config
}

func ParseGoogleCredentialsFromEnvironment() gcs.Credentials {
	// FUTURE: support for ADC? (https://cloud.google.com/docs/authentication/production)
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
