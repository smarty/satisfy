package packaging

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2/internal/model"
)

func TestDependencyResolverFixture(t *testing.T) {
	gunit.Run(new(DependencyResolverFixture), t)
}

type DependencyResolverFixture struct {
	*gunit.Fixture
	resolver         *DependencyResolver
	fileSystem       *inMemoryFileSystem
	integrityChecker *FakeIntegrityCheck
	packageInstaller *FakePackageInstaller
	dependency       model.Dependency
}

func (this *DependencyResolverFixture) Setup() {
	this.integrityChecker = &FakeIntegrityCheck{}
	this.fileSystem = newInMemoryFileSystem()
	this.packageInstaller = &FakePackageInstaller{}
	this.dependency = model.Dependency{
		PackageName:    "B/C",
		PackageVersion: "D",
		RemoteAddress:  model.URL(this.URL("gcs://A")),
		LocalDirectory: "local",
	}
	this.fileSystem.WriteFile("local/manifest_B|C.json", []byte("{}"))
}

func (this *DependencyResolverFixture) Resolve() error {
	this.resolver = NewDependencyResolver(this.fileSystem, this.integrityChecker, this.packageInstaller, this.dependency, nil)
	return this.resolver.Resolve(context.Background())
}

func (this *DependencyResolverFixture) TestFreshInstallation() {
	manifest := model.Manifest{
		Name:    "B/C",
		Version: "D",
		Archive: model.Archive{Filename: "archive-name"},
	}
	this.packageInstaller.remote = manifest

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.assertNewPackageInstalled(manifest.Name, this.dependency.PackageVersion)
}

func (this *DependencyResolverFixture) TestManifestInstallationFailure() {
	manifestErr := errors.New("manifest failure")
	this.packageInstaller.installManifestErr = manifestErr

	err := this.Resolve()

	this.So(errors.Is(err, manifestErr), should.BeTrue)
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
}

func (this *DependencyResolverFixture) TestManifestFileCannotBeRead() {
	readFileErr := errors.New("manifest file cannot be read")
	this.fileSystem.WriteFile("local/manifest_B___C.json", []byte("malformed json"))
	this.fileSystem.errReadFile["local/manifest_B___C.json"] = readFileErr

	err := this.Resolve()

	this.So(err, should.Resemble, readFileErr)
	this.So(this.packageInstaller.installManifestCounter, should.Equal, 0)
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
}

func (this *DependencyResolverFixture) TestManifestPresentButMalformed() {
	this.fileSystem.WriteFile("local/manifest_B___C.json", []byte("malformed json"))

	err := this.Resolve()

	this.So(err, should.NotBeNil)
	this.So(this.packageInstaller.installManifestCounter, should.Equal, 0)
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
}

func (this *DependencyResolverFixture) TestLocalManifestHasWrongPackageName() {
	this.prepareLocalPackageAndManifest("not "+this.dependency.PackageName, this.dependency.PackageVersion)

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.assertPreviouslyInstalledPackageUninstalled()
	this.assertNewPackageInstalled(this.dependency.PackageName, this.dependency.PackageVersion)
}

func (this *DependencyResolverFixture) TestLocalManifestHasWrongVersion() {
	this.prepareLocalPackageAndManifest(this.dependency.PackageName, "not"+this.dependency.PackageVersion)

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.assertPreviouslyInstalledPackageUninstalled()
	this.assertNewPackageInstalled(this.dependency.PackageName, this.dependency.PackageVersion)
}

func (this *DependencyResolverFixture) TestIntegrityCheckFailure() {
	localManifest := this.prepareLocalPackageAndManifest(this.dependency.PackageName, this.dependency.PackageVersion)
	this.integrityChecker.err = errors.New("integrity check failure")

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.assertPreviouslyInstalledPackageUninstalled()
	this.assertNewPackageInstalled(this.dependency.PackageName, this.dependency.PackageVersion)
	this.So(this.integrityChecker.localPath, should.Equal, this.dependency.LocalDirectory)
	this.So(this.integrityChecker.manifest, should.Resemble, localManifest)
}

func (this *DependencyResolverFixture) TestAlreadyInstalledCorrectly() {
	this.prepareLocalPackageAndManifest(this.dependency.PackageName, this.dependency.PackageVersion)

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents1")
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents2")
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents3")
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
	this.So(this.packageInstaller.installManifestCounter, should.Equal, 0)
}

func (this *DependencyResolverFixture) TestFinalInstallationFailed() {
	installError := errors.New("install package error")
	this.packageInstaller.installPackageErr = installError

	err := this.Resolve()

	this.So(errors.Is(err, installError), should.BeTrue)
}

func (this *DependencyResolverFixture) TestLatestIsAlreadyInstalled() {
	manifest := model.Manifest{
		Name:    "B/C",
		Version: "D",
	}
	this.packageInstaller.remoteLatest = manifest

	this.prepareLocalPackageAndManifest(this.dependency.PackageName, this.dependency.PackageVersion)
	this.dependency.PackageVersion = "latest"

	err := this.Resolve()

	this.So(err, should.BeNil)
}

func (this *DependencyResolverFixture) TestLocalPackageIsBehindLatest() {
	this.packageInstaller.remote = model.Manifest{
		Name:    "B/C",
		Version: "D",
	}

	this.packageInstaller.remoteLatest = model.Manifest{
		Name:    "B/C",
		Version: "E",
	}

	this.prepareLocalPackageAndManifest(this.dependency.PackageName, this.dependency.PackageVersion)
	this.dependency.PackageVersion = "latest"

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.assertPreviouslyInstalledPackageUninstalled()
	this.assertNewPackageInstalled(this.packageInstaller.remote.Name, "E")
}

func (this *DependencyResolverFixture) TestLatestFreshInstallation() {
	manifest := model.Manifest{
		Name:    "B/C",
		Version: "D",
		Archive: model.Archive{Filename: "archive-name"},
	}
	this.packageInstaller.remote = manifest
	this.dependency.PackageVersion = "latest"
	version := manifest.Version

	err := this.Resolve()

	this.assertLatestPackageInstalled(err, manifest.Name, version)
}

func (this *DependencyResolverFixture) assertLatestPackageInstalled(err error, name, version string) {
	this.So(err, should.BeNil)
	this.So(this.packageInstaller.installed, should.Resemble, this.packageInstaller.remote)
	this.So(this.packageInstaller.manifestRequest, should.Resemble, model.InstallationRequest{
		RemoteAddress: this.URL("gcs://A/B/C/manifest.json"),
		LocalPath:     "local",
		PackageName:   name,
	})
	this.So(this.packageInstaller.packageRequest, should.Resemble, model.InstallationRequest{
		RemoteAddress: this.URL(fmt.Sprintf("gcs://A/B/C/%s/archive", version)),
		LocalPath:     "local",
		PackageName:   "",
	})
}
func (this *DependencyResolverFixture) TestLatestManifestFailsToDownloadPreservesLocalInstallation() {
	downloadErr := errors.New("network unavailable")
	this.prepareLocalPackageAndManifest(this.dependency.PackageName, this.dependency.PackageVersion)
	this.dependency.PackageVersion = "latest"
	this.packageInstaller.downloadError = downloadErr

	err := this.Resolve()

	this.So(errors.Is(err, downloadErr), should.BeTrue)
	this.So(this.packageInstaller.installManifestCounter, should.Equal, 0)
	this.assertPreviousInstallationPreserved()
}

func (this *DependencyResolverFixture) TestReinstallManifestFailurePreservesLocalInstallation() {
	manifestErr := errors.New("network unavailable")
	this.prepareLocalPackageAndManifest(this.dependency.PackageName, "not"+this.dependency.PackageVersion)
	this.packageInstaller.installManifestErr = manifestErr

	err := this.Resolve()

	this.So(errors.Is(err, manifestErr), should.BeTrue)
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
	this.assertPreviousInstallationPreserved()
}

func (this *DependencyResolverFixture) TestTagFreshInstallation() {
	manifest := model.Manifest{
		Name:    "B/C",
		Version: "D",
		Archive: model.Archive{Filename: "archive-name"},
	}
	this.packageInstaller.remote = manifest
	this.dependency.PackageVersion = "stable"
	this.packageInstaller.errsByAddress = map[string]error{
		"gcs://A/B/C/stable/manifest.json": this.notFound("gcs://A/B/C/stable/manifest.json"),
	}
	this.packageInstaller.manifestsByAddress = map[string]model.Manifest{
		"gcs://A/B/C/manifest.json": {Name: "B/C", Version: "E", Tags: []model.Tag{{Name: "stable", Version: "D"}}},
	}

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.assertNewPackageInstalled(manifest.Name, "D")
}

func (this *DependencyResolverFixture) TestTagAlreadyInstalledCorrectly() {
	this.prepareLocalPackageAndManifest(this.dependency.PackageName, "D")
	this.dependency.PackageVersion = "stable"
	this.packageInstaller.errsByAddress = map[string]error{
		"gcs://A/B/C/stable/manifest.json": this.notFound("gcs://A/B/C/stable/manifest.json"),
	}
	this.packageInstaller.manifestsByAddress = map[string]model.Manifest{
		"gcs://A/B/C/manifest.json": {Name: "B/C", Version: "E", Tags: []model.Tag{{Name: "stable", Version: "D"}}},
	}

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.So(this.packageInstaller.installManifestCounter, should.Equal, 0)
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents1")
}

func (this *DependencyResolverFixture) TestTagPointsToDifferentVersion() {
	this.prepareLocalPackageAndManifest(this.dependency.PackageName, "D")
	this.dependency.PackageVersion = "stable"
	this.packageInstaller.remote = model.Manifest{Name: "B/C", Version: "E"}
	this.packageInstaller.errsByAddress = map[string]error{
		"gcs://A/B/C/stable/manifest.json": this.notFound("gcs://A/B/C/stable/manifest.json"),
	}
	this.packageInstaller.manifestsByAddress = map[string]model.Manifest{
		"gcs://A/B/C/manifest.json": {Name: "B/C", Version: "E", Tags: []model.Tag{{Name: "stable", Version: "E"}}},
	}

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.assertPreviouslyInstalledPackageUninstalled()
	this.assertNewPackageInstalled("B/C", "E")
}

func (this *DependencyResolverFixture) TestUnknownVersionOrTagPreservesLocalInstallation() {
	this.prepareLocalPackageAndManifest(this.dependency.PackageName, "D")
	this.dependency.PackageVersion = "no-such-version-or-tag"
	this.packageInstaller.errsByAddress = map[string]error{
		"gcs://A/B/C/no-such-version-or-tag/manifest.json": this.notFound("gcs://A/B/C/no-such-version-or-tag/manifest.json"),
	}
	this.packageInstaller.manifestsByAddress = map[string]model.Manifest{
		"gcs://A/B/C/manifest.json": {Name: "B/C", Version: "E", Tags: []model.Tag{{Name: "stable", Version: "D"}}},
	}

	err := this.Resolve()

	this.So(err, should.NotBeNil)
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents1")
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents2")
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents3")
}

func (this *DependencyResolverFixture) TestLiteralVersionShadowsTagOfSameName() {
	this.prepareLocalPackageAndManifest(this.dependency.PackageName, "D")
	this.dependency.PackageVersion = "stable"
	this.packageInstaller.remote = model.Manifest{Name: "B/C", Version: "stable"}
	this.packageInstaller.manifestsByAddress = map[string]model.Manifest{
		"gcs://A/B/C/stable/manifest.json": {Name: "B/C", Version: "stable"},
		"gcs://A/B/C/manifest.json":        {Name: "B/C", Version: "E", Tags: []model.Tag{{Name: "stable", Version: "D"}}},
	}

	err := this.Resolve()

	this.So(err, should.BeNil)
	this.assertPreviouslyInstalledPackageUninstalled()
	this.assertNewPackageInstalled("B/C", "stable")
}

func (this *DependencyResolverFixture) TestVersionProbeFailurePreservesLocalInstallation() {
	probeErr := errors.New("remote storage unavailable")
	this.prepareLocalPackageAndManifest(this.dependency.PackageName, "D")
	this.dependency.PackageVersion = "E"
	this.packageInstaller.errsByAddress = map[string]error{
		"gcs://A/B/C/E/manifest.json": probeErr,
	}

	err := this.Resolve()

	this.So(errors.Is(err, probeErr), should.BeTrue)
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents1")
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents2")
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents3")
}

func (this *DependencyResolverFixture) TestTagResolutionFailsWhenRootManifestUnavailable() {
	rootErr := errors.New("root manifest unavailable")
	this.dependency.PackageVersion = "stable"
	this.packageInstaller.errsByAddress = map[string]error{
		"gcs://A/B/C/stable/manifest.json": this.notFound("gcs://A/B/C/stable/manifest.json"),
		"gcs://A/B/C/manifest.json":        rootErr,
	}

	err := this.Resolve()

	this.So(errors.Is(err, rootErr), should.BeTrue)
	this.So(this.packageInstaller.installPackageCounter, should.Equal, 0)
}

func (this *DependencyResolverFixture) notFound(address string) error {
	return model.NewStatusCodeError(http.StatusNotFound, []int{http.StatusOK}, this.URL(address))
}

func (this *DependencyResolverFixture) assertNewPackageInstalled(name, version string) {
	this.So(this.packageInstaller.installed, should.Resemble, this.packageInstaller.remote)
	this.So(this.packageInstaller.manifestRequest, should.Resemble, model.InstallationRequest{
		RemoteAddress: this.URL(fmt.Sprintf("gcs://A/B/C/%s/manifest.json", version)),
		LocalPath:     "local",
		PackageName:   name,
	})
	this.So(this.packageInstaller.packageRequest, should.Resemble, model.InstallationRequest{
		RemoteAddress: this.URL(fmt.Sprintf("gcs://A/B/C/%s/archive", version)),
		LocalPath:     "local",
	})
}

func (this *DependencyResolverFixture) prepareLocalPackageAndManifest(
	packageName string, packageVersion string,
) model.Manifest {
	manifest := model.Manifest{
		Name:    packageName,
		Version: packageVersion,
		Archive: model.Archive{
			Filename: "archive",
			Contents: []model.ArchiveItem{
				{Path: "contents1"},
				{Path: "contents2"},
				{Path: "contents3"},
			},
		},
	}
	raw, _ := json.Marshal(manifest)
	this.fileSystem.WriteFile("local/manifest_B___C.json", raw)
	this.fileSystem.WriteFile("local/contents1", []byte("contents1"))
	this.fileSystem.WriteFile("local/contents2", []byte("contents2"))
	this.fileSystem.WriteFile("local/contents3", []byte("contents3"))
	var stored model.Manifest // as read back from disk
	_ = json.Unmarshal(raw, &stored)
	return stored
}

func (this *DependencyResolverFixture) assertPreviousInstallationPreserved() {
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents1")
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents2")
	this.So(this.fileSystem.fileSystem, should.ContainKey, "local/contents3")
}

func (this *DependencyResolverFixture) assertPreviouslyInstalledPackageUninstalled() {
	this.So(this.fileSystem.fileSystem, should.NotContainKey, "local/contents1")
	this.So(this.fileSystem.fileSystem, should.NotContainKey, "local/contents2")
	this.So(this.fileSystem.fileSystem, should.NotContainKey, "local/contents3")
}

func (this *DependencyResolverFixture) URL(address string) url.URL {
	parsed, err := url.Parse(address)
	this.So(err, should.BeNil)
	return *parsed
}

///////////////////////////////////////////////////////////////////////////////////////////////

type FakePackageInstaller struct {
	remote                 model.Manifest
	remoteLatest           model.Manifest
	installed              model.Manifest
	manifestRequest        model.InstallationRequest
	packageRequest         model.InstallationRequest
	installManifestErr     error
	installPackageErr      error
	installManifestCounter int
	installPackageCounter  int
	downloadError          error
	manifestsByAddress     map[string]model.Manifest
	errsByAddress          map[string]error
}

func (this *FakePackageInstaller) DownloadManifest(_ context.Context, address url.URL) (manifest model.Manifest, err error) {
	if err, found := this.errsByAddress[address.String()]; found {
		return model.Manifest{}, err
	}
	if manifest, found := this.manifestsByAddress[address.String()]; found {
		return manifest, nil
	}
	return this.remoteLatest, this.downloadError
}

func (this *FakePackageInstaller) ManifestExists(_ context.Context, address url.URL) (bool, error) {
	if err, found := this.errsByAddress[address.String()]; found {
		if model.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (this *FakePackageInstaller) InstallManifest(_ context.Context, request model.InstallationRequest) (manifest model.Manifest, err error) {
	this.installManifestCounter++
	this.manifestRequest = request
	if err, found := this.errsByAddress[request.RemoteAddress.String()]; found {
		return model.Manifest{}, err
	}
	if manifest, found := this.manifestsByAddress[request.RemoteAddress.String()]; found {
		return manifest, nil
	}
	return this.remote, this.installManifestErr
}

func (this *FakePackageInstaller) InstallPackage(_ context.Context, manifest model.Manifest, request model.InstallationRequest) error {
	this.installPackageCounter++
	this.installed = manifest
	this.packageRequest = request
	return this.installPackageErr
}
