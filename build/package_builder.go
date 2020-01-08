package build

import (
	"bitbucket.org/smartystreets/satisfy/contracts"
)

type PackageBuilder struct {
	storage  contracts.FileSystem
	archive  contracts.ArchiveWriter
}

func NewPackageBuilder(storage contracts.FileSystem, archive contracts.ArchiveWriter) *PackageBuilder {
	return &PackageBuilder{
		storage:  storage,
		archive:  archive,
	}
}

func (this *PackageBuilder) Build() error {
	return nil
}

func (this *PackageBuilder) Contents() []contracts.ArchiveItem {
	return nil
}
