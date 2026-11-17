package satisfy

import (
	"errors"

	"github.com/smarty/satisfy/v2/contracts"
)

var (
	// ErrPackageExists is returned by Upload when the package version is
	// already present on remote storage and overwriting was not requested.
	ErrPackageExists = contracts.ErrPackageExists

	// ErrNotFound matches (with errors.Is) failures caused by a remote object
	// that does not exist, such as an unknown package or version.
	ErrNotFound = contracts.ErrNotFound

	// ErrInvalidConfig matches (with errors.Is) failures caused by incomplete
	// or inconsistent input.
	ErrInvalidConfig = errors.New("invalid configuration")
)
