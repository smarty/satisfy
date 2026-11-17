package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"

	"github.com/smarty/satisfy/v2/internal/model"
)

const stdinPath = "_STDIN_"

// downloadCommandConfig is the configuration for the default (download)
// command: the library configuration plus the options only the CLI uses.
type downloadCommandConfig struct {
	MaxRetry          int
	Concurrency       int
	QuickVerification bool
	Dependencies      model.DependencyListing
	JSONPath          string
	ShowProgress      bool
}

func parseDownloadConfig(args []string, stdin io.Reader, stderr io.Writer) (config downloadCommandConfig, err error) {
	flags := flag.NewFlagSet("satisfy", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.IntVar(&config.MaxRetry,
		"max-retry",
		5,
		"How many times to retry attempts to download packages.",
	)
	flags.IntVar(&config.Concurrency,
		"concurrency",
		runtime.NumCPU(),
		"How many packages to download and install at the same time.",
	)
	flags.BoolVar(&config.QuickVerification,
		"quick",
		true,
		"When set to false, perform full file content validation on installed packages.",
	)
	flags.BoolVar(&config.ShowProgress,
		"progress",
		true,
		"Displays progress stats as files are added or extracted.",
	)
	flags.StringVar(&config.JSONPath,
		"json",
		stdinPath,
		"Path to file with dependency listing or, if equal to _STDIN_, read from stdin.",
	)

	flags.Usage = func() {
		output := flags.Output()
		_, _ = fmt.Fprintln(output, "Usage of satisfy:")
		flags.PrintDefaults()
		_, _ = fmt.Fprintln(output)
		_, _ = fmt.Fprintln(output, "  Package names may be passed as non-flag arguments and will serve as a filter "+
			"against the provided dependency listing.")
		_, _ = fmt.Fprintln(output)
		_, _ = fmt.Fprintln(output, "  The satisfy tool also provides these subcommands:")
		_, _ = fmt.Fprintln(output, "	check	Has package@version already been uploaded according to json config?")
		_, _ = fmt.Fprintln(output, "	latest	Print the latest published version of a package to stdout.")
		_, _ = fmt.Fprintln(output, "	tags	List or modify the tags of a package.")
		_, _ = fmt.Fprintln(output, "	upload	Upload package contents according to json config.")
		_, _ = fmt.Fprintln(output, "	version	Print the satisfy tool version to stdout.")
		_, _ = fmt.Fprintln(output)
	}

	if err = flags.Parse(args); err != nil {
		return downloadCommandConfig{}, err
	}
	if config.Concurrency < 1 {
		return downloadCommandConfig{}, errors.New("-concurrency must be at least 1")
	}

	config.Dependencies, err = loadDependencyListing(config.JSONPath, flags.Args(), stdin, stderr)
	if err != nil {
		return downloadCommandConfig{}, fmt.Errorf("unable to load dependency listing: %w", err)
	}
	return config, nil
}

func loadDependencyListing(path string, filter []string, stdin io.Reader, stderr io.Writer) (model.DependencyListing, error) {
	dependencies, err := readDependencyListing(path, stdin)
	if errors.Is(err, fs.ErrNotExist) {
		emitExampleDependenciesFile(stderr)
	}
	if err != nil {
		return model.DependencyListing{}, err
	}

	if err = dependencies.Validate(); err != nil {
		return model.DependencyListing{}, err
	}

	dependencies.Listing = Filter(dependencies.Listing, filter)

	if len(dependencies.Listing) == 0 {
		_, _ = fmt.Fprintln(stderr, "No dependencies provided. You can go about your business. Move along.")
		emitExampleDependenciesFile(stderr)
	}
	return dependencies, nil
}

func readDependencyListing(path string, stdin io.Reader) (listing model.DependencyListing, err error) {
	if path == stdinPath {
		return decodeDependencyListing(stdin)
	}
	file, err := os.Open(path)
	if err != nil {
		return listing, fmt.Errorf("could not open dependency file (%q): %w", path, err)
	}
	defer func() { _ = file.Close() }()
	return decodeDependencyListing(file)
}

func decodeDependencyListing(reader io.Reader) (listing model.DependencyListing, err error) {
	err = json.NewDecoder(reader).Decode(&listing)
	return listing, err
}

func emitExampleDependenciesFile(output io.Writer) {
	var listing model.DependencyListing
	listing.Listing = append(listing.Listing, model.Dependency{
		PackageName:    "example_package_name",
		PackageVersion: "0.0.1",
		RemoteAddress:  model.URL{Scheme: "gcs", Host: "bucket_name", Path: "/path/prefix"},
		LocalDirectory: "local/path",
	})
	raw, _ := json.MarshalIndent(listing, "", "  ") // the example always marshals
	_, _ = fmt.Fprintf(output, "Example json file:\n%s\n", raw)
}
