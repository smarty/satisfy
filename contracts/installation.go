package contracts

import "net/url"

type InstallationRequest struct {
	RemoteAddress url.URL
	LocalPath     string
}

type IntegrityCheck interface {
	Verify(manifest Manifest, localPath string) error
}

type PackageInstaller interface {
	InstallManifest(request InstallationRequest) (manifest Manifest, err error)
	InstallPackage(manifest Manifest, request InstallationRequest) error
}
