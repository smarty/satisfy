package configuration

import (
	"context"
	"flag"
	"fmt"

	"github.com/smarty/gcs"
	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/core"
	"github.com/smarty/satisfy/logging"
)

type DownloadConfiguration struct {
	Dependencies      contracts.DependencyListing
	GoogleCredentials gcs.Credentials
	MaxRetry          int
	QuickVerification bool
	ShowProgress      bool

	ctx                   context.Context
	gcsCredentialsReader  gcs.CredentialsReader
	jsonPath              string
	logger                *logging.Logger
	dependencyListingFunc func(path string) (contracts.DependencyListing, error)
}

// NewDownloadConfiguration creates a new [DownloadConfiguration] instance.
//
// Parameters:
//   - ctx: the context for managing cancellation and timeouts.
//   - dependencyListingFunc: a function to load the dependency listing from a
//     given path.
//   - gcsCredentialsReader: a reader for Google Cloud Storage credentials.
//   - logger: a logger for emitting messages.
//
// Returns:
//   - *DownloadConfiguration: a new download configuration instance.
func NewDownloadConfiguration(
	ctx context.Context,
	dependencyListingFunc func(path string) (contracts.DependencyListing, error),
	gcsCredentialsReader gcs.CredentialsReader,
	logger *logging.Logger,
) *DownloadConfiguration {
	return &DownloadConfiguration{
		ctx:                   ctx,
		dependencyListingFunc: dependencyListingFunc,
		gcsCredentialsReader:  gcsCredentialsReader,
		logger:                logger,
	}
}

// Parse processes command-line arguments to populate the configuration.
//
// Parameters:
//   - args: the command-line arguments to parse.
//
// Returns:
//   - error: an error if parsing fails; otherwise, nil.
func (this *DownloadConfiguration) Parse(args []string) (err error) {
	var flags *flag.FlagSet
	flags, err = this.parseFlags(args)
	if err != nil {
		return err
	}

	this.Dependencies, err = this.loadDependencyListing(this.jsonPath, flags.Args())
	if err != nil {
		this.logger.LogLine(logging.Warning, "Unable to load dependency listing: %v", err)
		return err
	}

	this.GoogleCredentials, err = this.gcsCredentialsReader.Read(this.ctx, this.Dependencies.Credentials)
	return err
}

func (this *DownloadConfiguration) loadDependencyListing(path string, filter []string) (contracts.DependencyListing, error) {
	dependencies, err := this.dependencyListingFunc(path)
	if err != nil {
		return contracts.DependencyListing{}, err
	}

	err = dependencies.Validate()
	if err != nil {
		return contracts.DependencyListing{}, err
	}

	dependencies.Listing = core.Filter(dependencies.Listing, filter)
	if len(dependencies.Listing) == 0 {
		this.logger.LogLine(logging.Warning, "No dependencies provided. You can go about your business. Move along.")
		return dependencies, contracts.ErrNoDependenciesMatch
	}

	return dependencies, nil
}

func (this *DownloadConfiguration) parseFlags(args []string) (flags *flag.FlagSet, err error) {
	flags = flag.NewFlagSet("satisfy", flag.ContinueOnError)
	flags.SetOutput(this.logger.WriterErr())
	flags.IntVar(&this.MaxRetry,
		"max-retry",
		5,
		"How many times to retry attempts to download packages.",
	)
	flags.BoolVar(&this.QuickVerification,
		"quick",
		true,
		"When set to false, perform full file content validation on installed packages.",
	)
	flags.BoolVar(&this.ShowProgress,
		"progress",
		true,
		"Displays progress stats as files are extracted from the archive.",
	)
	flags.StringVar(&this.jsonPath,
		"json",
		StdInPath,
		fmt.Sprintf("Path to file with dependency listing or, if equal to %q, read from stdin.", StdInPath),
	)

	flags.Usage = func() {
		this.logger.LogLineClean("Usage of %s:", flags.Name())
		flags.PrintDefaults()
		this.logger.LogLineClean("")
		this.logger.LogLineClean("  Package names may be passed as non-flag arguments and will serve as a filter " +
			"against the provided dependency listing.")
		this.logger.LogLineClean("  The satisfy tool also provides 2 additional subcommands:")
		this.logger.LogLineClean("")
		this.logger.LogLineClean("	check	Has package@version already been uploaded according to json config?")
		this.logger.LogLineClean("	upload	Upload package contents according to json config.")
		this.logger.LogLineClean("")
	}

	err = flags.Parse(args)
	if err != nil {
		this.logger.LogLine(logging.Warning, "Unable to parse command line flags: %v", err)
		return flags, err
	}

	return flags, nil
}
