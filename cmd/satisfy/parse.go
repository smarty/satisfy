package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"iter"
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
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}

	fmt.Fprintf(os.Stderr, "Example json file: %s\n", string(raw))
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
		return dependencies, contracts.ErrNoDependenciesMatch
	}

	return dependencies, nil
}

func newVaultCredentialsReader() gcs.CredentialsReader {
	return gcs.NewCredentialsReader(
		gcs.CredentialOptions.VaultServer(os.Getenv("VAULT_ADDR"), os.Getenv("VAULT_TOKEN")),
	)
}

func parseCheck(args []string) (contracts.CheckConfiguration, iter.Seq2[contracts.Event, error]) {
	flags := flag.NewFlagSet("satisfy check", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	var jsonPath string
	var maxRetry int
	var overwrite bool

	flags.StringVar(&jsonPath, "json", stdInPath,
		fmt.Sprintf("Path to config file or, if equal to %q, read from stdin.", stdInPath))
	flags.IntVar(&maxRetry, "max-retry", 5, "HTTP max retry.")
	flags.BoolVar(&overwrite, "overwrite", false,
		"When set, always upload package, even when it already exists at specified remote location.")

	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", flags.Name())
		flags.PrintDefaults()
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "exit code 0: success")
		fmt.Fprintln(os.Stderr, "exit code 1: general failure (see stderr for details)")
		fmt.Fprintln(os.Stderr, "exit code 2: package has already been uploaded")
	}

	if err := flags.Parse(args); err != nil {
		return contracts.CheckConfiguration{}, eventErrSeq(contracts.Event{
			Type: contracts.EventWarning, Message: fmt.Sprintf("Unable to parse command line flags: %v", err),
		}, err)
	}

	pkgConfig, err := readPackageConfig(jsonPath)
	if err != nil {
		return contracts.CheckConfiguration{}, eventErrSeq(contracts.Event{
			Type: contracts.EventFailure, Message: fmt.Sprintf("Error parsing configuration file: [%v]", err),
		}, err)
	}

	credReader := newVaultCredentialsReader()
	creds, err := credReader.Read(context.Background(), "")
	if err != nil {
		return contracts.CheckConfiguration{}, eventErrSeq(contracts.Event{
			Type: contracts.EventFailure, Message: fmt.Sprintf("Google authentication failed: [%v]", err),
		}, err)
	}

	if err = validatePackageConfig(pkgConfig, maxRetry); err != nil {
		return contracts.CheckConfiguration{}, errSeq(err)
	}

	return contracts.NewCheckConfiguration(creds, credReader, pkgConfig,
		contracts.CheckMaxRetry(maxRetry),
		contracts.CheckOverwrite(overwrite),
	), func(yield func(contracts.Event, error) bool) {}
}

func parseDownload(args []string) (contracts.DownloadConfiguration, iter.Seq2[contracts.Event, error]) {
	flags := flag.NewFlagSet("satisfy", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

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
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", flags.Name())
		flags.PrintDefaults()
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  Package names may be passed as non-flag arguments and will serve as a filter "+
			"against the provided dependency listing.")
		fmt.Fprintln(os.Stderr, "  The satisfy tool also provides 2 additional subcommands:")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "\tcheck\tHas package@version already been uploaded according to json config?")
		fmt.Fprintln(os.Stderr, "\tupload\tUpload package contents according to json config.")
		fmt.Fprintln(os.Stderr, "")
	}

	if err := flags.Parse(args); err != nil {
		return contracts.DownloadConfiguration{}, eventErrSeq(contracts.Event{
			Type: contracts.EventWarning, Message: fmt.Sprintf("Unable to parse command line flags: %v", err),
		}, err)
	}

	deps, err := loadDependencyListing(jsonPath, flags.Args())
	if errors.Is(err, contracts.ErrNoDependenciesMatch) {
		return contracts.DownloadConfiguration{}, func(yield func(contracts.Event, error) bool) {
			if !yield(contracts.Event{
				Type:    contracts.EventWarning,
				Message: "No dependencies provided. You can go about your business. Move along.",
			}, nil) {
				return
			}

			emitExampleDependenciesFile()
			yield(contracts.Event{}, err)
		}
	}

	if err != nil {
		return contracts.DownloadConfiguration{}, eventErrSeq(contracts.Event{
			Type: contracts.EventWarning, Message: fmt.Sprintf("Unable to load dependency listing: %v", err),
		}, err)
	}

	creds, err := gcs.NewCredentialsReader().Read(context.Background(), deps.Credentials)
	if err != nil {
		return contracts.DownloadConfiguration{}, errSeq(err)
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
	), func(yield func(contracts.Event, error) bool) {}
}

func parseUpload(args []string) (contracts.UploadConfiguration, iter.Seq2[contracts.Event, error]) {
	flags := flag.NewFlagSet("satisfy upload", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

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
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", flags.Name())
		flags.PrintDefaults()
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "exit code 0: success")
		fmt.Fprintln(os.Stderr, "exit code 1: general failure (see stderr for details)")
		fmt.Fprintln(os.Stderr, "exit code 2: package has already been uploaded")
	}

	if err := flags.Parse(args); err != nil {
		return contracts.UploadConfiguration{}, eventErrSeq(contracts.Event{
			Type: contracts.EventWarning, Message: fmt.Sprintf("Unable to parse command line flags: %v", err),
		}, err)
	}

	pkgConfig, err := readPackageConfig(jsonPath)
	if err != nil {
		return contracts.UploadConfiguration{}, eventErrSeq(contracts.Event{
			Type: contracts.EventFailure, Message: fmt.Sprintf("Error parsing configuration file: [%v]", err),
		}, err)
	}

	credReader := newVaultCredentialsReader()
	creds, err := credReader.Read(context.Background(), "")
	if err != nil {
		return contracts.UploadConfiguration{}, eventErrSeq(contracts.Event{
			Type: contracts.EventFailure, Message: fmt.Sprintf("Google authentication failed: [%v]", err),
		}, err)
	}

	if err = validatePackageConfig(pkgConfig, maxRetry); err != nil {
		return contracts.UploadConfiguration{}, errSeq(err)
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
	), func(yield func(contracts.Event, error) bool) {}
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
