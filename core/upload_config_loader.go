package core

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/smartystreets/gcs"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type UploadConfigLoader struct {
	storage     contracts.FileReader
	environment contracts.Environment
	stdin       io.Reader
}

func NewUploadConfigLoader(
	storage contracts.FileReader,
	environment contracts.Environment, stdin io.Reader,
) *UploadConfigLoader {
	return &UploadConfigLoader{
		storage:     storage,
		environment: environment,
		stdin:       stdin,
	}
}

func (this *UploadConfigLoader) LoadConfig(name string, args []string) (config contracts.UploadConfig, err error) {
	config, err = this.parseCLI(name, args)
	if err != nil {
		return contracts.UploadConfig{}, err
	}

	config.PackageConfig, err = this.parseConfigFile(config.JSONPath)
	if err != nil {
		return contracts.UploadConfig{}, err
	}

	config.GoogleCredentials, err = this.parseGoogleCredentialsFromEnvironment()
	if err != nil {
		return contracts.UploadConfig{}, err
	}

	err = this.validateConfigJsonValues(config)
	if err != nil {
		return contracts.UploadConfig{}, err
	}

	return config, nil
}

func (this *UploadConfigLoader) parseCLI(name string, args []string) (config contracts.UploadConfig, err error) {
	flags := flag.NewFlagSet("satisfy "+name, flag.ContinueOnError)
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
	flags.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of satisfy %s:", name)
		flags.PrintDefaults()
		_, _ = fmt.Fprintln(os.Stderr, `
exit code 0: success
exit code 1: general failure (see stderr for details)
exit code 2: package has already been uploaded`)
	}
	err = flags.Parse(args)

	return config, err
}

func (this *UploadConfigLoader) parseConfigFile(path string) (config contracts.PackageConfig, err error) {
	data, err := this.readRawJSON(path)
	if err != nil {
		return contracts.PackageConfig{}, err
	}
	return config, json.Unmarshal(data, &config)
}

func (this *UploadConfigLoader) parseGoogleCredentialsFromEnvironment() (gcs.Credentials, error) {
	googleCredentialsPath, found := this.environment.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS")
	googleCredentialsPath = strings.TrimSpace(googleCredentialsPath)
	if !found || googleCredentialsPath == "" {
		return gcs.Credentials{}, errors.New("the GOOGLE_APPLICATION_CREDENTIALS is required")
	}
	data, err := this.storage.ReadFile(googleCredentialsPath)
	if err != nil {
		return gcs.Credentials{}, err
	}
	credentials, err := gcs.ParseCredentialsFromJSON(data)
	if err != nil {
		return gcs.Credentials{}, err
	}
	return credentials, nil
}

func (this *UploadConfigLoader) readRawJSON(path string) (data []byte, err error) {
	if path == "" {
		return nil, blankJSONPathErr
	}
	if path == "_STDIN_" {
		return ioutil.ReadAll(this.stdin)
	} else {
		return this.storage.ReadFile(path)
	}
}

func (this *UploadConfigLoader) validateConfigJsonValues(config contracts.UploadConfig) error {
	if config.MaxRetry < 0 {
		return maxRetryErr
	}
	if config.PackageConfig.CompressionAlgorithm == "" {
		return blankCompressionAlgorithmErr
	}
	if config.PackageConfig.SourceDirectory == "" {
		return blankSourceDirectoryErr
	}
	if config.PackageConfig.PackageName == "" {
		return blankPackageNameErr
	}
	if config.PackageConfig.PackageVersion == "" {
		return blankPackageVersionErr
	}
	if config.PackageConfig.RemoteAddressPrefix == nil {
		return nilRemoteAddressPrefixErr
	}
	return nil
}

var (
	maxRetryErr                  = errors.New("max-retry must be positive")
	blankJSONPathErr             = errors.New("json flag must be populated")
	blankCompressionAlgorithmErr = errors.New("compression algorithm should not be blank")
	blankSourceDirectoryErr      = errors.New("source directory should not be blank")
	blankPackageNameErr          = errors.New("package name should not be blank")
	blankPackageVersionErr       = errors.New("package version should not be blank")
	nilRemoteAddressPrefixErr    = errors.New("remote address prefix should not be nil")
)
