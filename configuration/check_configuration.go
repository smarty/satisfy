package configuration

import (
	"github.com/smarty/gcs"
)

// CheckConfiguration holds the settings needed to determine whether a package
// version already exists on remote storage.
type CheckConfiguration struct {
	GoogleCredentials gcs.Credentials
	CredentialReader  gcs.CredentialsReader
	MaxRetry          int
	Overwrite         bool
	PackageConfig     PackageConfig
}

// CheckOption configures a [CheckConfiguration].
type CheckOption func(*CheckConfiguration)

// CheckMaxRetry sets the maximum number of HTTP retry attempts.
//
// Parameters:
//   - n: the maximum number of retries; must be non-negative.
//
// Returns:
//   - CheckOption: the configured option.
func CheckMaxRetry(n int) CheckOption {
	return func(c *CheckConfiguration) { c.MaxRetry = n }
}

// CheckOverwrite controls whether the remote manifest check is skipped.
// When enabled, the check command reports success regardless of whether the
// package already exists on remote storage.
//
// Parameters:
//   - enabled: when true, skips the remote manifest check.
//
// Returns:
//   - CheckOption: the configured option.
func CheckOverwrite(enabled bool) CheckOption {
	return func(c *CheckConfiguration) { c.Overwrite = enabled }
}

// NewCheckConfiguration creates a [CheckConfiguration] with the provided
// credentials and package config, applying any supplied options over the
// following defaults:
//   - MaxRetry:  5
//   - Overwrite: false
//
// Parameters:
//   - credentials:   GCS credentials used to authenticate remote storage calls.
//   - credReader:    reader used to refresh credentials when access tokens expire.
//   - packageConfig: the package metadata describing what to check.
//   - opts:          zero or more options that override the defaults above.
//
// Returns:
//   - CheckConfiguration: the fully populated configuration value.
func NewCheckConfiguration(credentials gcs.Credentials, credReader gcs.CredentialsReader, packageConfig PackageConfig, opts ...CheckOption) CheckConfiguration {
	c := CheckConfiguration{
		GoogleCredentials: credentials,
		CredentialReader:  credReader,
		PackageConfig:     packageConfig,
		MaxRetry:          5,
	}
	for _, opt := range opts {
		opt(&c)
	}

	return c
}
