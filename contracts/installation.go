package contracts

import "net/url"

type InstallationRequest struct {
	RemoteAddress url.URL
	LocalPath     string
}

type IntegrityCheck interface {
	Verify(manifest Manifest) error
}
