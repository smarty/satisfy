package core

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/smartystreets/satisfy/contracts"
)

type UploadConfigLoader struct {
	parser  CredentialParser
	storage contracts.FileReader
	stdin   io.Reader
	stderr  io.Writer
}

func NewUploadConfigLoader(storage contracts.FileReader, env contracts.Environment, stdin io.Reader, stderr io.Writer) *UploadConfigLoader {
	return &UploadConfigLoader{
		parser:  NewGoogleCredentialParser(storage, env),
		storage: storage,
		stdin:   stdin,
		stderr:  stderr,
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

	config.GoogleCredentials, err = this.parser.Parse()
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

func (this *UploadConfigLoader) parseConfigFile(path string) (config contracts.PackageConfig, err error) {
	data, err := this.readRawJSON(path)
	if err != nil {
		return contracts.PackageConfig{}, err
	}
	return config, json.Unmarshal(data, &config)
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
