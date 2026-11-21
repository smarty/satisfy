package packaging

import (
	"context"

	"github.com/smarty/satisfy/v2/internal/model"
)

type CompoundIntegrityCheck struct {
	inners []model.IntegrityCheck
}

func NewCompoundIntegrityCheck(inners ...model.IntegrityCheck) *CompoundIntegrityCheck {
	return &CompoundIntegrityCheck{inners: inners}
}

func (this *CompoundIntegrityCheck) Verify(ctx context.Context, manifest model.Manifest, localPath string) error {
	for _, inner := range this.inners {
		err := inner.Verify(ctx, manifest, localPath)
		if err != nil {
			return err
		}
	}
	return nil
}
