package satisfy

import (
	"errors"

	"github.com/smarty/satisfy/v2/internal/model"
)

var (
	// ErrPackageExists is returned by Upload when the package version is
	// already present on remote storage and overwriting was not requested.
	ErrPackageExists = model.ErrPackageExists

	// ErrNotFound matches (with errors.Is) failures caused by a remote object
	// that does not exist, such as an unknown package or version.
	ErrNotFound = model.ErrNotFound

	// ErrInvalidConfig matches (with errors.Is) failures caused by incomplete
	// or inconsistent input.
	ErrInvalidConfig = errors.New("invalid configuration")
)

// StatusCode returns the HTTP status code of the first unexpected response
// from remote storage found in err's chain, and whether one was found. A
// 404 also matches ErrNotFound.
func StatusCode(err error) (int, bool) {
	var statusErr *model.StatusCodeError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode(), true
	}
	return 0, false
}
