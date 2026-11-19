package main

import (
	"context"
	"encoding/json/v2"
	"flag"
	"fmt"
	"io"

	"github.com/smarty/gcs"
	"github.com/smarty/satisfy/v2"
	"github.com/smarty/satisfy/v2/internal/model"
)

// tagsCommandConfig is the configuration for the tags modify command.
type tagsCommandConfig struct {
	MaxRetry          int
	GoogleCredentials gcs.Credentials
	Modification      model.TagModificationConfig
	JSONPath          string
}

type TagsConfigLoader struct {
	reader  gcs.CredentialsReader
	storage model.FileReader
	stdin   io.Reader
	stderr  io.Writer
}

func NewTagsConfigLoader(storage model.FileReader, env environmentVariables, stdin io.Reader, stderr io.Writer) *TagsConfigLoader {
	vaultAddress, _ := env.LookupEnv("VAULT_ADDR")
	vaultToken, _ := env.LookupEnv("VAULT_TOKEN")

	reader := gcs.NewCredentialsReader(
		gcs.CredentialOptions.VaultServer(vaultAddress, vaultToken),
		gcs.CredentialOptions.EnvironmentReader(env),
		gcs.CredentialOptions.FileReader(storage))

	return &TagsConfigLoader{
		reader:  reader,
		storage: storage,
		stdin:   stdin,
		stderr:  stderr,
	}
}

func (this *TagsConfigLoader) LoadConfig(ctx context.Context, args []string) (config tagsCommandConfig, err error) {
	config, err = this.parseCLI(args)
	if err != nil {
		return tagsCommandConfig{}, err
	}

	config.Modification, err = this.parseConfigFile(config.JSONPath)
	if err != nil {
		return tagsCommandConfig{}, fmt.Errorf("could not parse configuration file: %w", err)
	}

	config.GoogleCredentials, err = this.reader.Read(ctx, "")
	if err != nil {
		return tagsCommandConfig{}, fmt.Errorf("google authentication failed: %w", err)
	}

	err = this.validate(config)
	if err != nil {
		return tagsCommandConfig{}, err
	}

	return config, nil
}

func (this *TagsConfigLoader) parseCLI(args []string) (config tagsCommandConfig, err error) {
	flags := flag.NewFlagSet("satisfy tags", flag.ContinueOnError)
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
	flags.Usage = func() {
		_, _ = fmt.Fprintln(this.stderr, "Usage of satisfy tags:")
		flags.PrintDefaults()
		_, _ = fmt.Fprintln(this.stderr, `
exit code 0: success
exit code 1: general failure (see stderr for details)`)
	}
	err = flags.Parse(args)

	return config, err
}

func (this *TagsConfigLoader) parseConfigFile(path string) (config model.TagModificationConfig, err error) {
	data, err := readRawJSON(this.storage, this.stdin, path)
	if err != nil {
		return model.TagModificationConfig{}, err
	}
	return config, json.Unmarshal(data, &config)
}

func (this *TagsConfigLoader) validate(config tagsCommandConfig) error {
	if config.MaxRetry < 0 {
		return errMaxRetry
	}
	return satisfy.ValidateTagModification(config.Modification)
}
