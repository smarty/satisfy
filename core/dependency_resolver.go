package core

import (
	"bitbucket.org/smartystreets/satisfy/contracts"
)

type DependencyResolverFileSystem interface {
	contracts.FileChecker
	contracts.FileReader
}

type packageInstaller interface {
	InstallManifest(request contracts.InstallationRequest) (manifest contracts.Manifest, err error)
	InstallPackage(manifest contracts.Manifest, request contracts.InstallationRequest)
}

type DependencyResolver struct {
	fileSystem       DependencyResolverFileSystem
	integrityChecker contracts.IntegrityCheck
	packageInstaller packageInstaller
	dependency       contracts.Dependency
}

func NewDependencyResolver(
	fileSystem DependencyResolverFileSystem,
	integrityChecker contracts.IntegrityCheck,
	packageInstaller packageInstaller,
	dependency contracts.Dependency,
) *DependencyResolver {
	return &DependencyResolver{
		fileSystem:       fileSystem,
		integrityChecker: integrityChecker,
		packageInstaller: packageInstaller,
		dependency:       dependency,
	}
}

func (this *DependencyResolver) Resolve() error {
	manifest, err := this.packageInstaller.InstallManifest(contracts.InstallationRequest{
		RemoteAddress: this.dependency.ComposeRemoteAddress(contracts.RemoteManifestFilename),
		LocalPath:     this.dependency.LocalDirectory,
	})
	if err != nil {
		return err
	}
	this.packageInstaller.InstallPackage(manifest, contracts.InstallationRequest{
		RemoteAddress: this.dependency.ComposeRemoteAddress(contracts.RemoteArchiveFilename),
		LocalPath:     this.dependency.LocalDirectory,
	})

	return nil
}
