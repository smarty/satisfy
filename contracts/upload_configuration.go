package contracts

import (
	"io"

	"github.com/smarty/gcs"
)

// UploadConfiguration holds the settings needed to archive and upload a
// package to remote storage.
type UploadConfiguration struct {
	GoogleCredentials gcs.Credentials
	CredentialReader  gcs.CredentialsReader
	MaxRetry          int
	NewProgress       func(int64) io.WriteCloser
	Overwrite         bool
	PackageConfig     PackageConfig
}

// UploadOption configures an [UploadConfiguration].
type UploadOption func(*UploadConfiguration)

// NewUploadConfiguration creates an [UploadConfiguration] with the provided
// credentials and package config, applying any supplied options over the
// following defaults:
//   - MaxRetry:  5
//   - Overwrite: false
//
// Parameters:
//   - credentials:   GCS credentials used to authenticate remote storage calls.
//   - credReader:    reader used to refresh credentials when access tokens expire.
//   - packageConfig: the package metadata describing what to upload.
//   - opts:          zero or more options that override the defaults above.
//
// Returns:
//   - UploadConfiguration: the fully populated configuration value.
func NewUploadConfiguration(credentials gcs.Credentials, credReader gcs.CredentialsReader, packageConfig PackageConfig, opts ...UploadOption) UploadConfiguration {
	c := UploadConfiguration{
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

// UploadMaxRetry sets the maximum number of HTTP retry attempts.
//
// Parameters:
//   - n: the maximum number of retries; must be non-negative.
//
// Returns:
//   - UploadOption: the configured option.
func UploadMaxRetry(n int) UploadOption {
	return func(c *UploadConfiguration) { c.MaxRetry = n }
}

// UploadOverwrite controls whether the pre-upload existence check is skipped.
// When enabled, the package is uploaded regardless of whether it already exists
// on remote storage.
//
// Parameters:
//   - enabled: when true, skips the pre-upload manifest check.
//
// Returns:
//   - UploadOption: the configured option.
func UploadOverwrite(enabled bool) UploadOption {
	return func(c *UploadConfiguration) { c.Overwrite = enabled }
}

// UploadProgress sets the factory used to create a progress writer for each
// file added to the archive. The factory receives the file size and must
// return an io.WriteCloser; bytes written to it are counted for progress
// reporting. A nil or no-op factory disables progress output.
//
// Parameters:
//   - factory: called with the file size; returns a progress-tracking writer.
//
// Returns:
//   - UploadOption: the configured option.
func UploadProgress(factory func(int64) io.WriteCloser) UploadOption {
	return func(c *UploadConfiguration) { c.NewProgress = factory }
}
