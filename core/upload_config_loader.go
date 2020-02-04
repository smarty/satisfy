package core

import (
	"encoding/json"
	"flag"
	"io"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type UploadConfigLoader struct {
	storage     contracts.FileReader
	environment contracts.Environment
	stdin       io.Reader
}

func NewUploadConfigLoader(
	storage contracts.FileReader,
	environment contracts.Environment,
	stdin io.Reader,
) *UploadConfigLoader {
	return &UploadConfigLoader{
		storage:     storage,
		environment: environment,
		stdin:       stdin,
	}
}

func (this *UploadConfigLoader) LoadConfig(name string, args []string) (config contracts.UploadConfig, err error) {
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
	err = flags.Parse(args)

	if err != nil {
		return contracts.UploadConfig{}, err
	}

	path := config.JSONPath
	data := this.storage.ReadFile(path)
	json.Unmarshal(data, &config.PackageConfig)

	return config, nil
}
