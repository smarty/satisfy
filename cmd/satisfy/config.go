package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/smartystreets/gcs"

	"bitbucket.org/smartystreets/satisfy/cmd"
)

type Config struct {
	MaxRetry          int
	QuickVerification bool
	JSONPath          string
	GoogleCredentials gcs.Credentials
	packageFilter     []string
}

func parseConfig(args []string) (config Config) {
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
		_, _ = fmt.Fprintln(output, "  Package names may be passed as non-flag arguments and will serve as a filter " +
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

	config.GoogleCredentials = cmd.ParseGoogleCredentialsFromEnvironment()

	return config
}
