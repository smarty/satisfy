package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/smarty/gcs"
	"github.com/smarty/satisfy/cmd/archive_progress"
	"github.com/smarty/satisfy/contracts"
)

const stdInPath = "_STDIN_"

func decodeDependencyListing(reader io.Reader) (contracts.DependencyListing, error) {
	var listing contracts.DependencyListing
	err := json.NewDecoder(reader).Decode(&listing)
	return listing, err
}

func emitExampleDependenciesFile() {
	var listing contracts.DependencyListing
	listing.Listing = append(listing.Listing, contracts.Dependency{
		PackageName:    "example_package_name",
		PackageVersion: "0.0.1",
		RemoteAddress:  contracts.URL{Scheme: "gcs", Host: "bucket_name", Path: "/path/prefix"},
		LocalDirectory: "local/path",
	})

	raw, err := json.MarshalIndent(listing, "", "  ")
	if err != nil {
		logger.LogClean("%v", err)
	}

	logger.LogLineClean("Example json file: %s", string(raw))
}

func loadDependencyListing(path string, filter []string) (contracts.DependencyListing, error) {
	dependencies, err := readDependencyListing(path)
	if err != nil {
		return contracts.DependencyListing{}, err
	}

	if err = dependencies.Validate(); err != nil {
		return contracts.DependencyListing{}, err
	}

	dependencies.Listing = contracts.Filter(dependencies.Listing, filter)
	if len(dependencies.Listing) == 0 {
		logger.LogLine(contracts.Warning, "No dependencies provided. You can go about your business. Move along.")
		return dependencies, contracts.ErrNoDependenciesMatch
	}

	return dependencies, nil
}

func newVaultCredentialsReader() gcs.CredentialsReader {
	return gcs.NewCredentialsReader(
		gcs.CredentialOptions.VaultServer(os.Getenv("VAULT_ADDR"), os.Getenv("VAULT_TOKEN")),
	)
}

func parseCheck(args []string) (contracts.CheckConfiguration, error) {
	flags := flag.NewFlagSet("satisfy check", flag.ContinueOnError)
	flags.SetOutput(logger.WriterErr())

	var jsonPath string
	var maxRetry int
	var overwrite bool

	flags.StringVar(&jsonPath, "json", stdInPath,
		fmt.Sprintf("Path to config file or, if equal to %q, read from stdin.", stdInPath))
	flags.IntVar(&maxRetry, "max-retry", 5, "HTTP max retry.")
	flags.BoolVar(&overwrite, "overwrite", false,
		"When set, always upload package, even when it already exists at specified remote location.")

	flags.Usage = func() {
		logger.LogLineClean("Usage of %s:", flags.Name())
		flags.PrintDefaults()
		logger.LogLineClean("")
		logger.LogLineClean("exit code 0: success")
		logger.LogLineClean("exit code 1: general failure (see stderr for details)")
		logger.LogLineClean("exit code 2: package has already been uploaded")
	}

	if err := flags.Parse(args); err != nil {
		logger.LogLine(contracts.Warning, "Unable to parse command line flags: %v", err)
		return contracts.CheckConfiguration{}, err
	}

	pkgConfig, err := readPackageConfig(jsonPath)
	if err != nil {
		logger.LogLine(contracts.Error, "Error parsing configuration file: %v", err)
		return contracts.CheckConfiguration{}, err
	}

	if err = validatePackageConfig(pkgConfig, maxRetry); err != nil {
		return contracts.CheckConfiguration{}, err
	}

	credReader := newVaultCredentialsReader()
	creds, err := credReader.Read(context.Background(), "")
	if err != nil {
		logger.LogLine(contracts.Error, "Google authentication failed: %v", err)
		return contracts.CheckConfiguration{}, err
	}

	return contracts.NewCheckConfiguration(creds, credReader, pkgConfig,
		contracts.CheckMaxRetry(maxRetry),
		contracts.CheckOverwrite(overwrite),
	), nil
}

func parseDownload(args []string) (contracts.DownloadConfiguration, error) {
	flags := flag.NewFlagSet("satisfy", flag.ContinueOnError)
	flags.SetOutput(logger.WriterErr())

	var jsonPath string
	var maxRetry int
	var quickVerification bool
	var showProgress bool

	flags.IntVar(&maxRetry, "max-retry", 5, "How many times to retry attempts to download packages.")
	flags.BoolVar(&quickVerification, "quick", true,
		"When set to false, perform full file content validation on installed packages.")
	flags.BoolVar(&showProgress, "progress", true,
		"Displays progress stats as files are extracted from the archive.")
	flags.StringVar(&jsonPath, "json", stdInPath,
		fmt.Sprintf("Path to dependency listing or, if equal to %q, read from stdin.", stdInPath))

	flags.Usage = func() {
		logger.LogLineClean("Usage of %s:", flags.Name())
		flags.PrintDefaults()
		logger.LogLineClean("")
		logger.LogLineClean("  Package names may be passed as non-flag arguments and will serve as a filter " +
			"against the provided dependency listing.")
		logger.LogLineClean("  The satisfy tool also provides 2 additional subcommands:")
		logger.LogLineClean("")
		logger.LogLineClean("	check	Has package@version already been uploaded according to json config?")
		logger.LogLineClean("	upload	Upload package contents according to json config.")
		logger.LogLineClean("")
	}

	if err := flags.Parse(args); err != nil {
		logger.LogLine(contracts.Warning, "Unable to parse command line flags: %v", err)
		return contracts.DownloadConfiguration{}, err
	}

	deps, err := loadDependencyListing(jsonPath, flags.Args())
	if errors.Is(err, contracts.ErrNoDependenciesMatch) {
		emitExampleDependenciesFile()
		return contracts.DownloadConfiguration{}, err
	}

	if err != nil {
		logger.LogLine(contracts.Warning, "Unable to load dependency listing: %v", err)
		return contracts.DownloadConfiguration{}, err
	}

	creds, err := gcs.NewCredentialsReader().Read(context.Background(), deps.Credentials)
	if err != nil {
		return contracts.DownloadConfiguration{}, err
	}

	var downloadProgress func(int64) io.WriteCloser
	if showProgress {
		downloadProgress = func(size int64) io.WriteCloser {
			return archive_progress.NewArchiveProgressCounter(size, func(archived, total string, done bool) {
				if done {
					fmt.Printf("\nDone extracting %s.\n", archived)
				} else {
					fmt.Printf("\033[2K\rExtracted %s of %s.", archived, total)
				}
			})
		}
	}

	return contracts.NewDownloadConfiguration(creds, deps,
		contracts.DownloadMaxRetry(maxRetry),
		contracts.DownloadQuickVerification(quickVerification),
		contracts.DownloadProgress(downloadProgress),
	), nil
}

func parseUpload(args []string) (contracts.UploadConfiguration, error) {
	flags := flag.NewFlagSet("satisfy upload", flag.ContinueOnError)
	flags.SetOutput(logger.WriterErr())

	var jsonPath string
	var maxRetry int
	var overwrite bool
	var showProgress bool

	flags.StringVar(&jsonPath, "json", stdInPath,
		fmt.Sprintf("Path to config file or, if equal to %q, read from stdin.", stdInPath))
	flags.IntVar(&maxRetry, "max-retry", 5, "HTTP max retry.")
	flags.BoolVar(&overwrite, "overwrite", false,
		"When set, always upload package, even when it already exists at specified remote location.")
	flags.BoolVar(&showProgress, "progress", true,
		"Displays progress stats as files are added to the archive.")

	flags.Usage = func() {
		logger.LogLineClean("Usage of %s:", flags.Name())
		flags.PrintDefaults()
		logger.LogLineClean("")
		logger.LogLineClean("exit code 0: success")
		logger.LogLineClean("exit code 1: general failure (see stderr for details)")
		logger.LogLineClean("exit code 2: package has already been uploaded")
	}

	if err := flags.Parse(args); err != nil {
		logger.LogLine(contracts.Warning, "Unable to parse command line flags: %v", err)
		return contracts.UploadConfiguration{}, err
	}

	pkgConfig, err := readPackageConfig(jsonPath)
	if err != nil {
		logger.LogLine(contracts.Error, "Error parsing configuration file: [%v]", err)
		return contracts.UploadConfiguration{}, err
	}

	credReader := newVaultCredentialsReader()
	creds, err := credReader.Read(context.Background(), "")
	if err != nil {
		logger.LogLine(contracts.Error, "Google authentication failed: [%v]", err)
		return contracts.UploadConfiguration{}, err
	}

	if err = validatePackageConfig(pkgConfig, maxRetry); err != nil {
		return contracts.UploadConfiguration{}, err
	}

	var uploadProgress func(int64) io.WriteCloser
	if showProgress {
		uploadProgress = func(size int64) io.WriteCloser {
			return archive_progress.NewArchiveProgressCounter(size, func(archived, total string, done bool) {
				if done {
					fmt.Printf("\nArchived %s of %s.\n", archived, total)
				} else {
					fmt.Printf("\033[2K\rArchived %s of %s.", archived, total)
				}
			})
		}
	}

	return contracts.NewUploadConfiguration(creds, credReader, pkgConfig,
		contracts.UploadMaxRetry(maxRetry),
		contracts.UploadOverwrite(overwrite),
		contracts.UploadProgress(uploadProgress),
	), nil
}

func readDependencyListing(path string) (contracts.DependencyListing, error) {
	if path == stdInPath {
		return decodeDependencyListing(os.Stdin)
	}

	file, err := os.Open(path)
	if os.IsNotExist(err) {
		emitExampleDependenciesFile()
		return contracts.DependencyListing{}, fmt.Errorf("specified dependency file (%q) not found: %w", path, err)
	}

	if err != nil {
		return contracts.DependencyListing{}, fmt.Errorf("could not open specified dependency file (%q): %w", path, err)
	}

	defer func() { _ = file.Close() }()
	return decodeDependencyListing(file)
}

func readPackageConfig(path string) (contracts.PackageConfig, error) {
	var data []byte
	var err error
	if path == stdInPath {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}

	if err != nil {
		return contracts.PackageConfig{}, fmt.Errorf("could not read config file (%q): %w", path, err)
	}

	var config contracts.PackageConfig
	if err = json.Unmarshal(data, &config); err != nil {
		return contracts.PackageConfig{}, err
	}

	return config, nil
}

func validatePackageConfig(config contracts.PackageConfig, maxRetry int) error {
	if maxRetry < 0 {
		return contracts.ErrMaxRetry
	}

	if config.CompressionAlgorithm == "" {
		return contracts.ErrBlankCompressionAlgorithm
	}

	if config.SourceDirectory == "" && config.SourceFile == "" && config.SourcePath == "" {
		return contracts.ErrBlankSourceDirectory
	}

	if config.PackageName == "" {
		return contracts.ErrBlankPackageName
	}

	if config.PackageVersion == "" {
		return contracts.ErrBlankPackageVersion
	}

	if config.RemoteAddressPrefix == nil {
		return contracts.ErrNilRemoteAddressPrefix
	}

	return nil
}
