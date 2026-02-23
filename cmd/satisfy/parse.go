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
	"github.com/smarty/satisfy/configuration"
)

const stdInPath = "_STDIN_"

func decodeDependencyListing(reader io.Reader) (configuration.DependencyListing, error) {
	var listing configuration.DependencyListing
	err := json.NewDecoder(reader).Decode(&listing)
	return listing, err
}

func emitExampleDependenciesFile() {
	var listing configuration.DependencyListing
	listing.Listing = append(listing.Listing, configuration.Dependency{
		PackageName:    "example_package_name",
		PackageVersion: "0.0.1",
		RemoteAddress:  configuration.URL{Scheme: "gcs", Host: "bucket_name", Path: "/path/prefix"},
		LocalDirectory: "local/path",
	})

	raw, err := json.MarshalIndent(listing, "", "  ")
	if err != nil {
		logger.LogClean("%v", err)
	}

	logger.LogLineClean("Example json file: %s", string(raw))
}

func loadDependencyListing(path string, filter []string) (configuration.DependencyListing, error) {
	dependencies, err := readDependencyListing(path)
	if err != nil {
		return configuration.DependencyListing{}, err
	}

	if err = dependencies.Validate(); err != nil {
		return configuration.DependencyListing{}, err
	}

	dependencies.Listing = configuration.Filter(dependencies.Listing, filter)
	if len(dependencies.Listing) == 0 {
		logger.LogLine(configuration.Warning, "No dependencies provided. You can go about your business. Move along.")
		return dependencies, configuration.ErrNoDependenciesMatch
	}

	return dependencies, nil
}

func newVaultCredentialsReader() gcs.CredentialsReader {
	return gcs.NewCredentialsReader(
		gcs.CredentialOptions.VaultServer(os.Getenv("VAULT_ADDR"), os.Getenv("VAULT_TOKEN")),
	)
}

func parseCheck(args []string) (configuration.CheckConfiguration, error) {
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
		logger.LogLine(configuration.Warning, "Unable to parse command line flags: %v", err)
		return configuration.CheckConfiguration{}, err
	}

	pkgConfig, err := readPackageConfig(jsonPath)
	if err != nil {
		logger.LogLine(configuration.Error, "Error parsing configuration file: %v", err)
		return configuration.CheckConfiguration{}, err
	}

	if err = validatePackageConfig(pkgConfig, maxRetry); err != nil {
		return configuration.CheckConfiguration{}, err
	}

	credReader := newVaultCredentialsReader()
	creds, err := credReader.Read(context.Background(), "")
	if err != nil {
		logger.LogLine(configuration.Error, "Google authentication failed: %v", err)
		return configuration.CheckConfiguration{}, err
	}

	return configuration.NewCheckConfiguration(creds, credReader, pkgConfig,
		configuration.CheckMaxRetry(maxRetry),
		configuration.CheckOverwrite(overwrite),
	), nil
}

func parseDownload(args []string) (configuration.DownloadConfiguration, error) {
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
		logger.LogLine(configuration.Warning, "Unable to parse command line flags: %v", err)
		return configuration.DownloadConfiguration{}, err
	}

	deps, err := loadDependencyListing(jsonPath, flags.Args())
	if errors.Is(err, configuration.ErrNoDependenciesMatch) {
		emitExampleDependenciesFile()
		return configuration.DownloadConfiguration{}, err
	}

	if err != nil {
		logger.LogLine(configuration.Warning, "Unable to load dependency listing: %v", err)
		return configuration.DownloadConfiguration{}, err
	}

	creds, err := gcs.NewCredentialsReader().Read(context.Background(), deps.Credentials)
	if err != nil {
		return configuration.DownloadConfiguration{}, err
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

	return configuration.NewDownloadConfiguration(creds, deps,
		configuration.DownloadMaxRetry(maxRetry),
		configuration.DownloadQuickVerification(quickVerification),
		configuration.DownloadProgress(downloadProgress),
	), nil
}

func parseUpload(args []string) (configuration.UploadConfiguration, error) {
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
		logger.LogLine(configuration.Warning, "Unable to parse command line flags: %v", err)
		return configuration.UploadConfiguration{}, err
	}

	pkgConfig, err := readPackageConfig(jsonPath)
	if err != nil {
		logger.LogLine(configuration.Error, "Error parsing configuration file: %v", err)
		return configuration.UploadConfiguration{}, err
	}

	credReader := newVaultCredentialsReader()
	creds, err := credReader.Read(context.Background(), "")
	if err != nil {
		logger.LogLine(configuration.Error, "Google authentication failed: [%v]", err)
		return configuration.UploadConfiguration{}, err
	}

	if err = validatePackageConfig(pkgConfig, maxRetry); err != nil {
		return configuration.UploadConfiguration{}, err
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

	return configuration.NewUploadConfiguration(creds, credReader, pkgConfig,
		configuration.UploadMaxRetry(maxRetry),
		configuration.UploadOverwrite(overwrite),
		configuration.UploadProgress(uploadProgress),
	), nil
}

func readDependencyListing(path string) (configuration.DependencyListing, error) {
	if path == stdInPath {
		return decodeDependencyListing(os.Stdin)
	}

	file, err := os.Open(path)
	if os.IsNotExist(err) {
		emitExampleDependenciesFile()
		return configuration.DependencyListing{}, fmt.Errorf("specified dependency file (%q) not found: %w", path, err)
	}

	if err != nil {
		return configuration.DependencyListing{}, fmt.Errorf("could not open specified dependency file (%q): %w", path, err)
	}

	defer func() { _ = file.Close() }()
	return decodeDependencyListing(file)
}

func readPackageConfig(path string) (configuration.PackageConfig, error) {
	var data []byte
	var err error
	if path == stdInPath {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}

	if err != nil {
		return configuration.PackageConfig{}, fmt.Errorf("could not read config file (%q): %w", path, err)
	}

	var config configuration.PackageConfig
	if err = json.Unmarshal(data, &config); err != nil {
		return configuration.PackageConfig{}, fmt.Errorf("could not parse config file (%q): %w", path, err)
	}

	return config, nil
}

func validatePackageConfig(config configuration.PackageConfig, maxRetry int) error {
	if maxRetry < 0 {
		return configuration.ErrMaxRetry
	}

	if config.CompressionAlgorithm == "" {
		return configuration.ErrBlankCompressionAlgorithm
	}

	if config.SourceDirectory == "" && config.SourceFile == "" && config.SourcePath == "" {
		return configuration.ErrBlankSourceDirectory
	}

	if config.PackageName == "" {
		return configuration.ErrBlankPackageName
	}

	if config.PackageVersion == "" {
		return configuration.ErrBlankPackageVersion
	}

	if config.RemoteAddressPrefix == nil {
		return configuration.ErrNilRemoteAddressPrefix
	}

	return nil
}
