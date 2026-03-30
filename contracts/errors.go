package contracts

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

var (
	ErrBlankCompressionAlgorithm = errors.New("compression algorithm should not be blank")
	ErrBlankPackageName          = errors.New("package name should not be blank")
	ErrBlankPackageVersion       = errors.New("package version should not be blank")
	ErrBlankSourceDirectory      = errors.New("'source path', 'source directory' or 'source file' must be provided")
	ErrMaxRetry                  = errors.New("max-retry must be positive")
	ErrNilRemoteAddressPrefix    = errors.New("remote address prefix should not be nil")
	ErrNoDependenciesMatch       = errors.New("no dependencies match the provided filter")
	ErrPackageExists             = errors.New("package already exists")
	ErrRetry                     = errors.New("retry")
)

type statusCodeError struct {
	actualStatusCode   int
	errorString        string
	expectedStatusCode []int
	remoteAddress      url.URL
}

func NewStatusCodeError(actual int, expected []int, remoteAddress url.URL) error {
	return &statusCodeError{actualStatusCode: actual, expectedStatusCode: expected, remoteAddress: remoteAddress}
}

func (this *statusCodeError) Error() string {
	if len(this.errorString) == 0 {
		var IDs []string
		for _, i := range this.expectedStatusCode {
			IDs = append(IDs, strconv.Itoa(i))
		}

		this.errorString = fmt.Sprintf(
			"expected status code: [%s] actual status code: [%d] remote address: [%s]",
			strings.Join(IDs, " or "), this.actualStatusCode, this.remoteAddress.String(),
		)
	}

	return this.errorString
}

func StatusCode(err error) (int, bool) {
	var target *statusCodeError
	if errors.As(err, &target) {
		return target.actualStatusCode, true
	}

	return 0, false
}
