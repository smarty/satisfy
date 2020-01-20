package core

import "bitbucket.org/smartystreets/satisfy/contracts"

type VersionIntegrityCheck struct {
	expectedVersion string
}

func (this *VersionIntegrityCheck) Verify(manifest contracts.Manifest) error {
	panic("implement me")
}
