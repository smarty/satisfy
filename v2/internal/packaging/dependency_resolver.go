package packaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/smarty/satisfy/v2/internal/model"
)

type DependencyResolverFileSystem interface {
	model.FileChecker
	model.FileReader
	model.Deleter
}

type DependencyResolver struct {
	fileSystem       DependencyResolverFileSystem
	integrityChecker model.IntegrityCheck
	packageInstaller model.PackageInstaller
	dependency       model.Dependency
	logger           *slog.Logger
}

func NewDependencyResolver(
	fileSystem DependencyResolverFileSystem,
	integrityChecker model.IntegrityCheck,
	packageInstaller model.PackageInstaller,
	dependency model.Dependency,
	logger *slog.Logger,
) *DependencyResolver {
	return &DependencyResolver{
		fileSystem:       fileSystem,
		integrityChecker: integrityChecker,
		packageInstaller: packageInstaller,
		dependency:       dependency,
		logger:           model.LoggerOrDiscard(logger),
	}
}

func (this *DependencyResolver) Resolve(ctx context.Context) error {
	this.logger.Info("installing dependency", "package", this.dependency.Title())

	manifestPath := ComposeManifestPath(this.dependency.LocalDirectory, this.dependency.PackageName)
	if !this.localManifestExists(manifestPath) {
		return this.installPackage(ctx)
	}

	localManifest, err := this.loadLocalManifest(manifestPath)
	if err != nil {
		return err
	}

	installed, err := this.isInstalledCorrectly(ctx, localManifest)
	if err != nil {
		return err
	}
	if installed {
		return nil
	}

	if err = this.uninstallPackage(localManifest); err != nil {
		return fmt.Errorf("failed to uninstall previous package contents for %s: %w", this.dependency.Title(), err)
	}
	return this.installPackage(ctx)
}

func (this *DependencyResolver) loadLocalManifest(manifestPath string) (localManifest model.Manifest, err error) {
	file, err := this.fileSystem.ReadFile(manifestPath)
	if err != nil {
		return localManifest, err
	}
	err = json.Unmarshal(file, &localManifest)
	if err == nil {
		return localManifest, nil
	}
	return model.Manifest{}, fmt.Errorf(
		"existing manifest found but malformed at %q (%s);"+
			" the corresponding package must be uninstalled manually"+
			" before installation of %q at version %q can be attempted",
		manifestPath, err, this.dependency.PackageName, this.dependency.PackageVersion)
}

func (this *DependencyResolver) localManifestExists(manifestPath string) bool {
	_, err := this.fileSystem.Stat(manifestPath)
	return !os.IsNotExist(err)
}

func (this *DependencyResolver) isInstalledCorrectly(ctx context.Context, localManifest model.Manifest) (bool, error) {
	if localManifest.Name != this.dependency.PackageName {
		if strings.HasSuffix(localManifest.Name, "/"+this.dependency.PackageName) {
			// no-op
		} else {
			this.logger.Info("incorrect package installed; reinstalling",
				"installed", localManifest.Name, "package", this.dependency.Title())
			return false, nil
		}
	}
	if this.dependency.PackageVersion == "latest" && !this.localManifestIsLatest(ctx, localManifest) {
		this.logger.Info("incorrect version installed; reinstalling",
			"installed", localManifest.Version, "package", this.dependency.Title())
		return false, nil
	} else if this.dependency.PackageVersion != "latest" && localManifest.Version != this.dependency.PackageVersion {
		matchesTag, err := this.localManifestMatchesRequestedTag(ctx, localManifest)
		if err != nil {
			return false, err
		}
		if !matchesTag {
			this.logger.Info("incorrect version installed; reinstalling",
				"installed", localManifest.Version, "package", this.dependency.Title())
			return false, nil
		}
	}

	verifyErr := this.integrityChecker.Verify(localManifest, this.dependency.LocalDirectory)
	if verifyErr != nil {
		this.logger.Info("integrity check failed; reinstalling", "package", this.dependency.Title(), "error", verifyErr)
		return false, nil
	}

	this.logger.Info("dependency already installed", "package", this.dependency.Title())
	return true, nil
}

// localManifestMatchesRequestedTag is consulted when the requested version does
// not match the locally installed version. A remote manifest at the literal
// version path takes precedence over a tag of the same name; only when no such
// version exists is the requested string resolved against the root manifest tags.
func (this *DependencyResolver) localManifestMatchesRequestedTag(ctx context.Context, localManifest model.Manifest) (bool, error) {
	exists, err := this.packageInstaller.ManifestExists(ctx, this.dependency.ComposeRemoteAddress(model.RemoteManifestFilename))
	if err != nil {
		return false, fmt.Errorf("failed to check for manifest of %s: %w", this.dependency.Title(), err)
	}
	if exists {
		return false, nil
	}

	version, err := this.resolveTag(ctx)
	if err != nil {
		return false, err
	}
	this.dependency.PackageVersion = version
	return version == localManifest.Version, nil
}

// resolveTag translates the requested version into a concrete version by way of
// the tag listing in the root manifest.
func (this *DependencyResolver) resolveTag(ctx context.Context) (string, error) {
	requested := this.dependency.PackageVersion
	rootManifest, err := this.packageInstaller.DownloadManifest(ctx, this.dependency.ComposeLatestManifestRemoteAddress())
	if err != nil {
		return "", fmt.Errorf("failed to download root manifest while resolving %q for package %q: %w",
			requested, this.dependency.PackageName, err)
	}
	version, found := rootManifest.TagVersion(requested)
	if !found {
		return "", fmt.Errorf("no version or tag %q exists for package %q", requested, this.dependency.PackageName)
	}
	this.logger.Info("resolved tag", "tag", requested, "version", version, "package", this.dependency.PackageName)
	return version, nil
}

func (this *DependencyResolver) installPackage(ctx context.Context) error {
	this.logger.Info("downloading manifest", "package", this.dependency.Title())
	manifest, err := this.packageInstaller.InstallManifest(ctx, model.InstallationRequest{
		RemoteAddress: this.dependency.ComposeRemoteManifestAddress(),
		LocalPath:     this.dependency.LocalDirectory,
		PackageName:   this.dependency.PackageName,
	})
	if model.IsNotFound(err) && this.dependency.PackageVersion != "latest" {
		// No manifest exists at the literal version path; the requested
		// version may be a tag defined in the root manifest.
		var version string
		version, err = this.resolveTag(ctx)
		if err != nil {
			return fmt.Errorf("failed to install manifest for %s: %w", this.dependency.Title(), err)
		}
		this.dependency.PackageVersion = version
		manifest, err = this.packageInstaller.InstallManifest(ctx, model.InstallationRequest{
			RemoteAddress: this.dependency.ComposeRemoteManifestAddress(),
			LocalPath:     this.dependency.LocalDirectory,
			PackageName:   this.dependency.PackageName,
		})
	}
	if err != nil {
		return fmt.Errorf("failed to install manifest for %s: %w", this.dependency.Title(), err)
	}
	this.logger.Info("downloading and extracting package contents", "package", this.dependency.Title())

	if this.dependency.PackageVersion == "latest" {
		this.dependency.PackageVersion = manifest.Version
	}

	// TODO:
	//  Manifest archive should always be used during download/install instead of model.RemoteArchiveFilename.
	//    The model.RemoteArchiveFilename is to be used during the creation of a manifest, but once the manifest
	//    exists, we can change the archive filename in the contracts and all previously uploaded manifests using the
	//    older name are still recognized and understood.

	err = this.packageInstaller.InstallPackage(ctx, manifest, model.InstallationRequest{
		RemoteAddress: this.dependency.ComposeRemoteAddress(model.RemoteArchiveFilename),
		LocalPath:     this.dependency.LocalDirectory,
	})
	if err != nil {
		return fmt.Errorf("failed to install package contents for %s: %w", this.dependency.Title(), err)
	}

	this.logger.Info("dependency installed", "package", this.dependency.Title())
	return nil
}

func (this *DependencyResolver) uninstallPackage(manifest model.Manifest) (err error) {
	for _, item := range manifest.Archive.Contents {
		path := filepath.Join(this.dependency.LocalDirectory, item.Path)
		err = errors.Join(err, ignoreNotExist(this.fileSystem.Delete(path)))
	}
	return err
}

func (this *DependencyResolver) localManifestIsLatest(ctx context.Context, manifest model.Manifest) bool {
	remoteManifest, err := this.packageInstaller.DownloadManifest(ctx, this.dependency.ComposeRemoteManifestAddress())
	if err != nil {
		this.logger.Warn("failed to download the latest manifest", "package", this.dependency.Title(), "error", err)
		return false
	}
	this.dependency.PackageVersion = remoteManifest.Version
	return remoteManifest.Version == manifest.Version
}
