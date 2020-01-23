package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/smartystreets/gcs"

	"bitbucket.org/smartystreets/satisfy/cmd"
)

type Config struct {
	MaxRetry          int
	Verify            bool
	JSONPath          string
	GoogleCredentials gcs.Credentials
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
	flag.StringVar(&config.JSONPath,
		"json",
		"_STDIN_",
		"Path to file with dependency listing or, if equal to _STDIN_, read from stdin.",
	)

	flag.Usage = func() {
		output := flag.CommandLine.Output()
		_, _ = fmt.Fprintf(output, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		_, _ = fmt.Fprintln(output)
		_, _ = fmt.Fprintln(output, "  The satisfy tool also provides 2 additional subcommands:")
		_, _ = fmt.Fprintln(output, "	check	Has package@version already been uploaded according to json config?")
		_, _ = fmt.Fprintln(output, "	upload	Upload package contents according to json config.")
		_, _ = fmt.Fprintln(output)
	}

	flag.Parse()

	config.GoogleCredentials = cmd.ParseGoogleCredentialsFromEnvironment()

	return config
}
