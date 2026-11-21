package model

import (
	"context"
	"net/url"
)

type InstallationRequest struct {
	RemoteAddress url.URL
	LocalPath     string
	PackageName   string
}

type IntegrityCheck interface {
	Verify(ctx context.Context, manifest Manifest, localPath string) error
}

type PackageInstaller interface {
	DownloadManifest(ctx context.Context, remoteAddress url.URL) (manifest Manifest, err error)
	ManifestExists(ctx context.Context, remoteAddress url.URL) (exists bool, err error)
	InstallManifest(ctx context.Context, request InstallationRequest) (manifest Manifest, err error)
	InstallPackage(ctx context.Context, manifest Manifest, request InstallationRequest) error
}
