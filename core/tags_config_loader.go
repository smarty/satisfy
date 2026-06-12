package core

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"

	"github.com/smarty/gcs"

	"github.com/smarty/satisfy/contracts"
)

type TagsConfigLoader struct {
	reader  gcs.CredentialsReader
	storage contracts.FileReader
	stdin   io.Reader
	stderr  io.Writer
}

func NewTagsConfigLoader(storage contracts.FileReader, env contracts.Environment, stdin io.Reader, stderr io.Writer) *TagsConfigLoader {
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

func (this *TagsConfigLoader) LoadConfig(args []string) (config contracts.TagsConfig, err error) {
	config, err = this.parseCLI(args)
	if err != nil {
		return contracts.TagsConfig{}, err
	}

	config.Modification, err = this.parseConfigFile(config.JSONPath)
	if err != nil {
		log.Printf("[Error] Error parsing configuration file: [%s]", err)
		return contracts.TagsConfig{}, err
	}

	config.GoogleCredentials, err = this.reader.Read(context.Background(), "")
	if err != nil {
		log.Printf("[Error] Google authentication failed: [%s]", err)
		return contracts.TagsConfig{}, err
	}

	err = this.validate(config)
	if err != nil {
		return contracts.TagsConfig{}, err
	}

	return config, nil
}

func (this *TagsConfigLoader) parseCLI(args []string) (config contracts.TagsConfig, err error) {
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

func (this *TagsConfigLoader) parseConfigFile(path string) (config contracts.TagModificationConfig, err error) {
	data, err := readRawJSON(this.storage, this.stdin, path)
	if err != nil {
		return contracts.TagModificationConfig{}, err
	}
	return config, json.Unmarshal(data, &config)
}

func (this *TagsConfigLoader) validate(config contracts.TagsConfig) error {
	if config.MaxRetry < 0 {
		return maxRetryErr
	}
	modification := config.Modification
	if modification.PackageName == "" {
		return blankPackageNameErr
	}
	if modification.RemoteAddress == nil {
		return nilRemoteAddressPrefixErr
	}
	if len(modification.Add) == 0 && len(modification.Delete) == 0 {
		return noTagModificationsErr
	}

	added := make(map[string]struct{})
	for _, tag := range modification.Add {
		if err := validateTagName(tag.Name); err != nil {
			return err
		}
		if tag.Version == "" {
			return blankTagVersionErr
		}
		if _, found := added[tag.Name]; found {
			return duplicateTagNameErr
		}
		added[tag.Name] = struct{}{}
	}

	deleted := make(map[string]struct{})
	for _, tag := range modification.Delete {
		if tag.Name == "" {
			return blankTagNameErr
		}
		if _, found := added[tag.Name]; found {
			return conflictingTagNameErr
		}
		if _, found := deleted[tag.Name]; found {
			return duplicateTagNameErr
		}
		deleted[tag.Name] = struct{}{}
	}
	return nil
}
