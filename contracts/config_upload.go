package contracts

import (
	"github.com/smarty/gcs"
	"github.com/smarty/satisfy/configuration"
)

type UploadConfig struct {
	MaxRetry          int
	CredentialReader  gcs.CredentialsReader
	GoogleCredentials gcs.Credentials
	JSONPath          string
	Overwrite         bool
	ShowProgress      bool
	PackageConfig     configuration.PackageConfig
}
