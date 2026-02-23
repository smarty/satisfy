package configuration

import "errors"

var (
	ErrBlankCompressionAlgorithm = errors.New("compression algorithm should not be blank")
	ErrBlankPackageName          = errors.New("package name should not be blank")
	ErrBlankPackageVersion       = errors.New("package version should not be blank")
	ErrBlankSourceDirectory      = errors.New("'source path', 'source directory' or 'source file' must be provided")
	ErrMaxRetry                  = errors.New("max-retry must be positive")
	ErrNilRemoteAddressPrefix    = errors.New("remote address prefix should not be nil")
	ErrNoDependenciesMatch       = errors.New("no dependencies match the provided filter")
)
