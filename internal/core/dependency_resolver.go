package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/internal/plumbing"
)

type DependencyResolverFileSystem interface {
	plumbing.FileChecker
	plumbing.FileReader
	plumbing.Deleter
}

type DependencyResolver struct {
	fileSystem       DependencyResolverFileSystem
	integrityChecker plumbing.IntegrityCheck
	packageInstaller plumbing.PackageInstaller
	dependency       contracts.Dependency
	emit             func(contracts.Event)
}

func NewDependencyResolver(
	fileSystem DependencyResolverFileSystem,
	integrityChecker plumbing.IntegrityCheck,
	packageInstaller plumbing.PackageInstaller,
	dependency contracts.Dependency,
	emit func(contracts.Event),
) *DependencyResolver {
	if emit == nil {
		emit = func(contracts.Event) {}
	}
	return &DependencyResolver{
		fileSystem:       fileSystem,
		integrityChecker: integrityChecker,
		packageInstaller: packageInstaller,
		dependency:       dependency,
		emit:             emit,
	}
}

func (this *DependencyResolver) Resolve() error {
	this.emit(contracts.Event{Type: contracts.EventInfo, Message: fmt.Sprintf("Installing dependency: %s", this.dependency.Title())})

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

func (this *DependencyResolver) loadLocalManifest(manifestPath string) (localManifest plumbing.Manifest, err error) {
	file, err := this.fileSystem.ReadFile(manifestPath)
	if err != nil {
		return localManifest, err
	}
	err = json.Unmarshal(file, &localManifest)
	if err == nil {
		return localManifest, nil
	}
	return plumbing.Manifest{}, fmt.Errorf(
		"existing manifest found but malformed at %q (%s);"+
			" the corresponding package must be uninstalled manually"+
			" before installation of %q at version %q can be attempted",
		manifestPath, err, this.dependency.PackageName, this.dependency.PackageVersion)
}

func (this *DependencyResolver) localManifestExists(manifestPath string) bool {
	_, err := this.fileSystem.Stat(manifestPath)
	return !os.IsNotExist(err)
}

func (this *DependencyResolver) isInstalledCorrectly(localManifest plumbing.Manifest) bool {
	if localManifest.Name != this.dependency.PackageName {
		if strings.HasSuffix(localManifest.Name, "/"+this.dependency.PackageName) {
			// no-op
		} else {
			this.emit(contracts.Event{Type: contracts.EventInfo, Message: fmt.Sprintf(
				"incorrect package installed (%s), proceeding to installation of specified package: %s",
				localManifest.Name, this.dependency.Title())})
			return false
		}
	}
	if this.dependency.PackageVersion == "latest" && !this.localManifestIsLatest(localManifest) {
		this.emit(contracts.Event{Type: contracts.EventInfo, Message: fmt.Sprintf(
			"incorrect version installed (%s), proceeding to installation of specified package: %s",
			localManifest.Version, this.dependency.Title())})
		return false
	} else if this.dependency.PackageVersion != "latest" && localManifest.Version != this.dependency.PackageVersion {
		this.emit(contracts.Event{Type: contracts.EventInfo, Message: fmt.Sprintf(
			"incorrect version installed (%s), proceeding to installation of specified package: %s",
			localManifest.Version, this.dependency.Title())})
		return false
	}

	verifyErr := this.integrityChecker.Verify(localManifest, this.dependency.LocalDirectory)
	if verifyErr != nil {
		this.emit(contracts.Event{Type: contracts.EventInfo, Message: fmt.Sprintf("%s in %s", verifyErr.Error(), this.dependency.Title())})
		return false
	}

	this.emit(contracts.Event{Type: contracts.EventInfo, Message: fmt.Sprintf("Dependency already installed: %s", this.dependency.Title())})
	return true
}

func (this *DependencyResolver) installPackage() error {
	this.emit(contracts.Event{Type: contracts.EventInfo, Message: fmt.Sprintf("Downloading manifest for %s", this.dependency.Title())})
	manifest, err := this.packageInstaller.InstallManifest(plumbing.InstallationRequest{
		RemoteAddress: this.dependency.ComposeRemoteManifestAddress(),
		LocalPath:     this.dependency.LocalDirectory,
		PackageName:   this.dependency.PackageName,
	})
	if err != nil {
		return fmt.Errorf("failed to install manifest for %s: %w", this.dependency.Title(), err)
	}
	this.emit(contracts.Event{Type: contracts.EventInfo, Message: fmt.Sprintf("Downloading and extracting package contents for %s", this.dependency.Title())})

	if this.dependency.PackageVersion == "latest" {
		this.dependency.PackageVersion = manifest.Version
	}

	// TODO:
	//  Manifest archive should always be used during download/install instead of configuration.RemoteArchiveFilename.
	//    The configuration.RemoteArchiveFilename is to be used during the creation of a manifest, but once the manifest
	//    exists, we can change the archive filename in the configuration and all previously uploaded manifests using the
	//    older name are still recognized and understood.

	err = this.packageInstaller.InstallPackage(manifest, plumbing.InstallationRequest{
		RemoteAddress: this.dependency.ComposeRemoteAddress(contracts.RemoteArchiveFilename),
		LocalPath:     this.dependency.LocalDirectory,
	})
	if err != nil {
		return fmt.Errorf("failed to install package contents for %s: %w", this.dependency.Title(), err)
	}

	this.emit(contracts.Event{Type: contracts.EventInfo, Message: fmt.Sprintf("Dependency installed: %s", this.dependency.Title())})
	return nil
}

func (this *DependencyResolver) uninstallPackage(manifest plumbing.Manifest) {
	for _, item := range manifest.Archive.Contents {
		_ = this.fileSystem.Delete(filepath.Join(this.dependency.LocalDirectory, item.Path))
	}
}

func (this *DependencyResolver) localManifestIsLatest(manifest plumbing.Manifest) bool {
	remoteManifest, err := this.packageInstaller.DownloadManifest(this.dependency.ComposeRemoteManifestAddress())
	if err != nil {
		this.emit(contracts.Event{Type: contracts.EventWarning, Message: fmt.Sprintf("Failed to download the latest manifest file: %v", err)})
		return false
	}
	this.dependency.PackageVersion = remoteManifest.Version
	return remoteManifest.Version == manifest.Version
}
