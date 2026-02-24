package core

import "github.com/smarty/satisfy/legacy_contracts"

type CompoundIntegrityCheck struct {
	inners []legacy_contracts.IntegrityCheck
}

func NewCompoundIntegrityCheck(inners ...legacy_contracts.IntegrityCheck) *CompoundIntegrityCheck {
	return &CompoundIntegrityCheck{inners: inners}
}

func (this *CompoundIntegrityCheck) Verify(manifest legacy_contracts.Manifest, localPath string) error {
	for _, inner := range this.inners {
		err := inner.Verify(manifest, localPath)
		if err != nil {
			return err
		}
	}
	return nil
}
