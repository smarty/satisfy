package core

import "github.com/smartystreets/satisfy/contracts"

type CompoundIntegrityCheck struct {
	inners []contracts.IntegrityCheck
}

func NewCompoundIntegrityCheck(inners ...contracts.IntegrityCheck) *CompoundIntegrityCheck {
	return &CompoundIntegrityCheck{inners: inners}
}

func (this *CompoundIntegrityCheck) Verify(manifest contracts.Manifest, localPath string) error {
	for _, inner := range this.inners {
		err := inner.Verify(manifest, localPath)
		if err != nil {
			return err
		}
	}
	return nil
}
