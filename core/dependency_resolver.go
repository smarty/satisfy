package core

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/smarty/satisfy/contracts"
)

type DependencyResolverFileSystem interface {
	contracts.FileChecker
	contracts.FileReader
	contracts.Deleter
}

type DependencyResolver struct {
	fileSystem       DependencyResolverFileSystem
	integrityChecker contracts.IntegrityCheck
	packageInstaller contracts.PackageInstaller
	dependency       contracts.Dependency
}

func NewDependencyResolver(
	fileSystem DependencyResolverFileSystem,
	integrityChecker contracts.IntegrityCheck,
	packageInstaller contracts.PackageInstaller,
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
	log.Printf("Installing dependency: %s", this.dependency.Title())

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
	file, err := this.fileSystem.ReadFile(manifestPath)
	if err != nil {
		return localManifest, err
	}
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
		if strings.HasSuffix(localManifest.Name, "/"+this.dependency.PackageName) {
			// no-op
		} else {
			log.Printf("incorrect package installed (%s), proceeding to installation of specified package: %s",
				localManifest.Name, this.dependency.Title())
			return false
		}
	}
	if this.dependency.PackageVersion == "latest" && !this.localManifestIsLatest(localManifest) {
		log.Printf("incorrect version installed (%s), proceeding to installation of specified package: %s",
			localManifest.Version, this.dependency.Title())
		return false
	} else if this.dependency.PackageVersion != "latest" && localManifest.Version != this.dependency.PackageVersion {
		log.Printf("incorrect version installed (%s), proceeding to installation of specified package: %s",
			localManifest.Version, this.dependency.Title())
		return false
	}

	verifyErr := this.integrityChecker.Verify(localManifest, this.dependency.LocalDirectory)
	if verifyErr != nil {
		log.Printf("%s in %s", verifyErr.Error(), this.dependency.Title())
		return false
	}

	log.Printf("Dependency already installed: %s", this.dependency.Title())
	return true
}

func (this *DependencyResolver) installPackage() error {
	log.Printf("Downloading manifest for %s", this.dependency.Title())
	manifest, err := this.packageInstaller.InstallManifest(contracts.InstallationRequest{
		RemoteAddress: this.dependency.ComposeRemoteManifestAddress(),
		LocalPath:     this.dependency.LocalDirectory,
		PackageName:   this.dependency.PackageName,
	})
	if err != nil {
		return fmt.Errorf("failed to install manifest for %s: %w", this.dependency.Title(), err)
	}
	log.Printf("Downloading and extracting package contents for %s", this.dependency.Title())

	if this.dependency.PackageVersion == "latest" {
		this.dependency.PackageVersion = manifest.Version
	}

	// TODO:
	//  Manifest archive should always be used during download/install instead of contracts.RemoteArchiveFilename.
	//    The contracts.RemoteArchiveFilename is to be used during the creation of a manifest, but once the manifest
	//    exists, we can change the archive filename in the contracts and all previously uploaded manifests using the
	//    older name are still recognized and understood.

	err = this.packageInstaller.InstallPackage(manifest, contracts.InstallationRequest{
		RemoteAddress: this.dependency.ComposeRemoteAddress(contracts.RemoteArchiveFilename),
		LocalPath:     this.dependency.LocalDirectory,
	})
	if err != nil {
		return fmt.Errorf("failed to install package contents for %s: %w", this.dependency.Title(), err)
	}

	log.Printf("Dependency installed: %s", this.dependency.Title())
	return nil
}

func (this *DependencyResolver) uninstallPackage(manifest contracts.Manifest) {
	for _, item := range manifest.Archive.Contents {
		this.fileSystem.Delete(filepath.Join(this.dependency.LocalDirectory, item.Path))
	}
}

func (this *DependencyResolver) localManifestIsLatest(manifest contracts.Manifest) bool {
	remoteManifest, err := this.packageInstaller.DownloadManifest(this.dependency.ComposeRemoteManifestAddress())
	if err != nil {
		log.Println("Failed to download the latest manifest file:", err)
		return false
	}
	this.dependency.PackageVersion = remoteManifest.Version
	return remoteManifest.Version == manifest.Version
}
