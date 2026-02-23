package configuration

import (
	"io"

	"github.com/smarty/gcs"
)

// DownloadConfiguration holds the settings needed to download and install
// package dependencies.
type DownloadConfiguration struct {
	Dependencies      DependencyListing
	GoogleCredentials gcs.Credentials
	MaxRetry          int
	NewProgress       func(int64) io.WriteCloser
	QuickVerification bool
}

// DownloadOption configures a [DownloadConfiguration].
type DownloadOption func(*DownloadConfiguration)

// DownloadMaxRetry sets the maximum number of HTTP retry attempts per package.
//
// Parameters:
//   - n: the maximum number of retries; must be non-negative.
//
// Returns:
//   - DownloadOption: the configured option.
func DownloadMaxRetry(n int) DownloadOption {
	return func(c *DownloadConfiguration) { c.MaxRetry = n }
}

// DownloadQuickVerification controls the integrity check strategy used for
// already-installed packages. When disabled, full file content (MD5) validation
// is performed in addition to the faster file-listing check.
//
// Parameters:
//   - enabled: when true, only the file listing is validated; when false, file
//     contents are also verified.
//
// Returns:
//   - DownloadOption: the configured option.
func DownloadQuickVerification(enabled bool) DownloadOption {
	return func(c *DownloadConfiguration) { c.QuickVerification = enabled }
}

// DownloadProgress sets the factory used to create a progress writer for each
// file extracted from an archive. The factory receives the file size and must
// return an io.WriteCloser; bytes written to it are counted for progress
// reporting. A nil or no-op factory disables progress output.
//
// Parameters:
//   - factory: called with the file size; returns a progress-tracking writer.
//
// Returns:
//   - DownloadOption: the configured option.
func DownloadProgress(factory func(int64) io.WriteCloser) DownloadOption {
	return func(c *DownloadConfiguration) { c.NewProgress = factory }
}

// NewDownloadConfiguration creates a [DownloadConfiguration] with the provided
// credentials and dependency listing, applying any supplied options over the
// following defaults:
//   - MaxRetry:          5
//   - QuickVerification: true
//
// Parameters:
//   - credentials:  GCS credentials used to authenticate remote storage calls.
//   - dependencies: the listing of packages to download and install.
//   - opts:         zero or more options that override the defaults above.
//
// Returns:
//   - DownloadConfiguration: the fully populated configuration value.
func NewDownloadConfiguration(credentials gcs.Credentials, dependencies DependencyListing, opts ...DownloadOption) DownloadConfiguration {
	c := DownloadConfiguration{
		GoogleCredentials: credentials,
		Dependencies:      dependencies,
		MaxRetry:          5,
		QuickVerification: true,
	}
	for _, opt := range opts {
		opt(&c)
	}

	return c
}
