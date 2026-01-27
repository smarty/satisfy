package configuration

import (
	"context"
	"flag"
	"fmt"

	"github.com/smarty/gcs"
	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/logging"
)

type CheckConfiguration struct {
	GoogleCredentials gcs.Credentials
	CredentialReader  gcs.CredentialsReader
	MaxRetry          int
	Overwrite         bool
	PackageConfig     contracts.PackageConfig

	ctx                  context.Context
	gcsCredentialsReader gcs.CredentialsReader
	jsonPath             string
	logger               *logging.Logger
	packageConfigFunc    func(path string) (contracts.PackageConfig, error)
}

// NewCheckConfiguration creates a new [CheckConfiguration] instance.
//
// Parameters:
//   - ctx: the context for managing cancellation and timeouts.
//   - packageConfigFunc: a function to load the package configuration from a
//     given path.
//   - gcsCredentialsReader: a reader for Google Cloud Storage credentials.
//   - logger: a logger for emitting messages.
//
// Returns:
//   - *CheckConfiguration: a new check configuration instance.
func NewCheckConfiguration(
	ctx context.Context,
	packageConfigFunc func(path string) (contracts.PackageConfig, error),
	gcsCredentialsReader gcs.CredentialsReader,
	logger *logging.Logger,
) *CheckConfiguration {
	return &CheckConfiguration{
		ctx:                  ctx,
		packageConfigFunc:    packageConfigFunc,
		gcsCredentialsReader: gcsCredentialsReader,
		logger:               logger,
	}
}

// Parse processes command-line arguments to populate the configuration.
//
// Parameters:
//   - args: the command-line arguments to parse.
//
// Returns:
//   - error: an error if parsing fails; otherwise, nil.
func (this *CheckConfiguration) Parse(args []string) (err error) {
	err = this.parseFlags(args)
	if err != nil {
		return err
	}

	this.PackageConfig, err = this.packageConfigFunc(this.jsonPath)
	if err != nil {
		this.logger.LogLine(logging.Error, "Error parsing configuration file: %v", err)
		return err
	}

	err = this.validatePackageConfig()
	if err != nil {
		return err
	}

	this.GoogleCredentials, err = this.gcsCredentialsReader.Read(this.ctx, "")
	if err != nil {
		this.logger.LogLine(logging.Error, "Google authentication failed: %v", err)
		return err
	}

	this.CredentialReader = this.gcsCredentialsReader
	return nil
}

func (this *CheckConfiguration) parseFlags(args []string) (err error) {
	flags := flag.NewFlagSet("satisfy check", flag.ContinueOnError)
	flags.SetOutput(this.logger.WriterErr())
	flags.StringVar(&this.jsonPath,
		"json",
		StdInPath,
		fmt.Sprintf("Path to file with config file or, if equal to %q, read from stdin.", StdInPath),
	)
	flags.IntVar(&this.MaxRetry,
		"max-retry",
		5,
		"HTTP max retry.",
	)
	flags.BoolVar(&this.Overwrite,
		"overwrite",
		false,
		"When set, always upload package, even when it already exists at specified remote location.",
	)

	flags.Usage = func() {
		this.logger.LogLineClean("Usage of %s:", flags.Name())
		flags.PrintDefaults()
		this.logger.LogLineClean("")
		this.logger.LogLineClean("exit code 0: success")
		this.logger.LogLineClean("exit code 1: general failure (see stderr for details)")
		this.logger.LogLineClean("exit code 2: package has already been uploaded")
	}

	err = flags.Parse(args)
	if err != nil {
		this.logger.LogLine(logging.Warning, "Unable to parse command line flags: %v", err)
		return err
	}

	return nil
}

func (this *CheckConfiguration) validatePackageConfig() error {
	if this.MaxRetry < 0 {
		return contracts.ErrMaxRetry
	}

	if this.PackageConfig.CompressionAlgorithm == "" {
		return contracts.ErrBlankCompressionAlgorithm
	}

	if this.PackageConfig.SourceDirectory == "" && this.PackageConfig.SourceFile == "" && this.PackageConfig.SourcePath == "" {
		return contracts.ErrBlankSourceDirectory
	}

	if this.PackageConfig.PackageName == "" {
		return contracts.ErrBlankPackageName
	}

	if this.PackageConfig.PackageVersion == "" {
		return contracts.ErrBlankPackageVersion
	}

	if this.PackageConfig.RemoteAddressPrefix == nil {
		return contracts.ErrNilRemoteAddressPrefix
	}

	return nil
}
