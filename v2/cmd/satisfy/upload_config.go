package main

import (
	"context"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/smarty/gcs"
	"github.com/smarty/satisfy/v2"
	"github.com/smarty/satisfy/v2/internal/model"
)

// uploadCommandConfig is the configuration for the upload and check
// commands: the library configuration plus the options only the CLI uses.
type uploadCommandConfig struct {
	MaxRetry          int
	CredentialReader  gcs.CredentialsReader
	GoogleCredentials gcs.Credentials
	Overwrite         bool
	PackageConfig     model.PackageConfig
	JSONPath          string
	ShowProgress      bool
}

type UploadConfigLoader struct {
	reader  gcs.CredentialsReader
	storage model.FileReader
	stdin   io.Reader
	stderr  io.Writer
}

func NewUploadConfigLoader(storage model.FileReader, env environmentVariables, stdin io.Reader, stderr io.Writer) *UploadConfigLoader {
	vaultAddress, _ := env.LookupEnv("VAULT_ADDR")
	vaultToken, _ := env.LookupEnv("VAULT_TOKEN")

	reader := gcs.NewCredentialsReader(
		gcs.CredentialOptions.VaultServer(vaultAddress, vaultToken),
		gcs.CredentialOptions.EnvironmentReader(env),
		gcs.CredentialOptions.FileReader(storage))

	return &UploadConfigLoader{
		reader:  reader,
		storage: storage,
		stdin:   stdin,
		stderr:  stderr,
	}
}

func (this *UploadConfigLoader) LoadConfig(ctx context.Context, name string, args []string) (config uploadCommandConfig, err error) {
	config, err = this.parseCLI(name, args)
	if err != nil {
		return uploadCommandConfig{}, err
	}

	config.PackageConfig, err = this.parseConfigFile(config.JSONPath)
	if err != nil {
		return uploadCommandConfig{}, fmt.Errorf("could not parse configuration file: %w", err)
	}

	config.GoogleCredentials, err = this.reader.Read(ctx, "")
	if err != nil {
		return uploadCommandConfig{}, fmt.Errorf("google authentication failed: %w", err)
	}

	err = this.validateConfigJsonValues(config)
	if err != nil {
		return uploadCommandConfig{}, err
	}

	// allow refreshing the token later which accommodates long compression times which cause the bearer token to expire
	config.CredentialReader = this.reader

	return config, nil
}

func (this *UploadConfigLoader) parseCLI(name string, args []string) (config uploadCommandConfig, err error) {
	flags := flag.NewFlagSet("satisfy "+name, flag.ContinueOnError)
	flags.SetOutput(this.stderr)
	flags.StringVar(&config.JSONPath,
		"json",
		"_STDIN_",
		"Path to file with config file or, if equal to _STDIN_, read from stdin.",
	)
	flags.IntVar(&config.MaxRetry,
		"max-retry",
		5,
		"HTTP max retry.",
	)
	flags.BoolVar(&config.Overwrite,
		"overwrite",
		false,
		"When set, always upload package, even when it already exists at specified remote location.",
	)
	flags.BoolVar(&config.ShowProgress,
		"progress",
		true,
		"Displays progress stats as files are added to the archive.",
	)
	flags.Usage = func() {
		_, _ = fmt.Fprintf(this.stderr, "Usage of satisfy %s:", name)
		flags.PrintDefaults()
		_, _ = fmt.Fprintln(this.stderr, `
exit code 0: success
exit code 1: general failure (see stderr for details)
exit code 2: package has already been uploaded`)
	}
	err = flags.Parse(args)

	return config, err
}

func (this *UploadConfigLoader) parseConfigFile(path string) (config model.PackageConfig, err error) {
	data, err := this.readRawJSON(path)
	if err != nil {
		return model.PackageConfig{}, err
	}
	return config, json.Unmarshal(data, &config)
}

func (this *UploadConfigLoader) readRawJSON(path string) (data []byte, err error) {
	return readRawJSON(this.storage, this.stdin, path)
}

func readRawJSON(storage model.FileReader, stdin io.Reader, path string) (data []byte, err error) {
	if path == "" {
		return nil, errBlankJSONPath
	}
	if path == "_STDIN_" {
		return io.ReadAll(stdin)
	} else {
		return storage.ReadFile(path)
	}
}

func (this *UploadConfigLoader) validateConfigJsonValues(config uploadCommandConfig) error {
	if config.MaxRetry < 0 {
		return errMaxRetry
	}
	return satisfy.ValidatePackageConfig(config.PackageConfig)
}

var (
	errMaxRetry      = errors.New("max-retry must be positive")
	errBlankJSONPath = errors.New("json flag must be populated")
)
