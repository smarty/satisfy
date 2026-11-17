package storage

import "net/http"

// maxIdleConnectionsPerHost keeps enough connections to Google Cloud Storage
// (a single host) open for concurrent installs to reuse.
const maxIdleConnectionsPerHost = 32

// NewHTTPClient returns a client that reuses connections (keep-alives on,
// HTTP/2 when the server offers it), starting from Go's default transport.
func NewHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = maxIdleConnectionsPerHost
	return &http.Client{Transport: transport}
}
