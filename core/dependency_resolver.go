package core

import (
	"encoding/json"
	"fmt"
	"os"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type DependencyResolverFileSystem interface {
	contracts.FileChecker
	contracts.FileReader
	contracts.Deleter
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
	manifestPath := ComposeManifestPath(this.dependency.LocalDirectory, this.dependency.PackageName)
	if !this.localManifestExists(manifestPath) {
		return this.installPackage()
	}
	localManifest, err := this.loadLocalManifest(manifestPath)
	if err != nil {
		return err
	}
	if this.isInstalledCorrectly(localManifest) {
		return nil
	}

	this.uninstallPackage(localManifest)
	return this.installPackage()
}

func (this *DependencyResolver) loadLocalManifest(manifestPath string) (localManifest contracts.Manifest, err error) {
	file := this.fileSystem.ReadFile(manifestPath)
	err = json.Unmarshal(file, &localManifest)
	if err == nil {
		return localManifest, nil
	}
	return contracts.Manifest{}, fmt.Errorf(
		"existing manifest found but malformed at %q (%s);"+
			" the corresponding package must be uninstalled manually"+
			" before installation of %q at version %q can be attempted",
		manifestPath, err, this.dependency.PackageName, this.dependency.PackageVersion)
}

func (this *DependencyResolver) localManifestExists(manifestPath string) bool {
	_, err := this.fileSystem.Stat(manifestPath)
	return !os.IsNotExist(err)
}

func (this *DependencyResolver) isInstalledCorrectly(localManifest contracts.Manifest) bool {
	if localManifest.Name != this.dependency.PackageName {
		return false
	}
	if localManifest.Version != this.dependency.PackageVersion {
		return false
	}
	//log.Printf("%s in %s", verifyErr.Error(), dependency.Title())

	if this.integrityChecker.Verify(localManifest, this.dependency.LocalDirectory) != nil {
		return false
	}
	return true
}

func (this *DependencyResolver) installPackage() error {
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

func (this *DependencyResolver) uninstallPackage(manifest contracts.Manifest) {
	for _, item := range manifest.Archive.Contents {
		this.fileSystem.Delete(item.Path)
	}
}
