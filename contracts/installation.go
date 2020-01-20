package contracts

import "net/url"

type InstallationRequest struct {
	RemoteAddress url.URL
	LocalPath     string
}
