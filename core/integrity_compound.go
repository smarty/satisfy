package core

import "bitbucket.org/smartystreets/satisfy/contracts"

type CompoundIntegrityCheck struct {
	inners []contracts.IntegrityCheck
}

func NewCompoundIntegrityCheck(inners ...contracts.IntegrityCheck) *CompoundIntegrityCheck {
	return &CompoundIntegrityCheck{inners: inners}
}

func (this *CompoundIntegrityCheck) Verify(manifest contracts.Manifest) error {
	for _, inner := range this.inners {
		err := inner.Verify(manifest)
		if err != nil {
			return err
		}
	}
	return nil
}
