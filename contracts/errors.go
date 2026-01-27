package contracts

import "errors"

var (
	ErrNoDependenciesMatch = errors.New("no dependencies match the provided filter")
)
