package configuration

import (
	"context"
	"flag"
	"fmt"

	"github.com/smarty/gcs"
	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/logging"
)

type UploadConfiguration struct {
	GoogleCredentials gcs.Credentials
	CredentialReader  gcs.CredentialsReader
	MaxRetry          int
	Overwrite         bool
	ShowProgress      bool
	PackageConfig     contracts.PackageConfig

	ctx                  context.Context
	gcsCredentialsReader gcs.CredentialsReader
	jsonPath             string
	logger               *logging.Logger
	packageConfigFunc    func(path string) (contracts.PackageConfig, error)
}

// NewUploadConfiguration creates a new [UploadConfiguration] instance.
//
// Parameters:
//   - ctx: the context for managing cancellation and timeouts.
//   - packageConfigFunc: a function to load the package configuration from a
//     given path.
//   - gcsCredentialsReader: a reader for Google Cloud Storage credentials.
//   - logger: a logger for emitting messages.
//
// Returns:
//   - *UploadConfiguration: a new upload configuration instance.
func NewUploadConfiguration(
	ctx context.Context,
	packageConfigFunc func(path string) (contracts.PackageConfig, error),
	gcsCredentialsReader gcs.CredentialsReader,
	logger *logging.Logger,
) *UploadConfiguration {
	return &UploadConfiguration{
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
func (this *UploadConfiguration) Parse(args []string) (err error) {
	err = this.parseFlags(args)
	if err != nil {
		return err
	}

	this.PackageConfig, err = this.packageConfigFunc(this.jsonPath)
	if err != nil {
		this.logger.LogLine(logging.Error, "Error parsing configuration file: %v", err)
		return err
	}

	this.GoogleCredentials, err = this.gcsCredentialsReader.Read(this.ctx, "")
	if err != nil {
		this.logger.LogLine(logging.Error, "Google authentication failed: [%v]", err)
		return err
	}

	err = this.validatePackageConfig()
	if err != nil {
		return err
	}

	this.CredentialReader = this.gcsCredentialsReader
	return nil
}

func (this *UploadConfiguration) parseFlags(args []string) (err error) {
	flags := flag.NewFlagSet("satisfy upload", flag.ContinueOnError)
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
	flags.BoolVar(&this.ShowProgress,
		"progress",
		true,
		"Displays progress stats as files are added to the archive.",
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

func (this *UploadConfiguration) validatePackageConfig() error {
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
