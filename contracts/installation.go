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
	DownloadManifest(remoteAddress url.URL) (manifest Manifest, err error)
	InstallManifest(request InstallationRequest) (manifest Manifest, err error)
	InstallPackage(manifest Manifest, request InstallationRequest) error
}
