package core

import "github.com/smarty/satisfy/internal/plumbing"

type CompoundIntegrityCheck struct {
	inners []plumbing.IntegrityCheck
}

func NewCompoundIntegrityCheck(inners ...plumbing.IntegrityCheck) *CompoundIntegrityCheck {
	return &CompoundIntegrityCheck{inners: inners}
}

func (this *CompoundIntegrityCheck) Verify(manifest plumbing.Manifest, localPath string) error {
	for _, inner := range this.inners {
		err := inner.Verify(manifest, localPath)
		if err != nil {
			return err
		}
	}
	return nil
}
