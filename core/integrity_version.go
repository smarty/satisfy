package core

import (
	"errors"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type VersionIntegrityCheck struct {
	desiredVersion string
}

func NewVersionIntegrityCheck(desiredVersion string) *VersionIntegrityCheck {
	return &VersionIntegrityCheck{desiredVersion: desiredVersion}
}

func (this *VersionIntegrityCheck) Verify(manifest contracts.Manifest) error {
	if manifest.Version != this.desiredVersion {
		return errVersionMismatch
	}
	return nil
}

var errVersionMismatch = errors.New("version mismatch")