package satisfy

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseURL parses a remote address such as "gcs://bucket/prefix". A bare
// bucket name, optionally followed by a path ("bucket" or "bucket/prefix"),
// is read as a gcs:// address. Any other scheme, or a missing bucket, fails
// with an error matching ErrInvalidConfig.
func ParseURL(raw string) (URL, error) {
	value := strings.TrimSpace(raw)
	if !strings.Contains(value, "://") {
		value = "gcs://" + strings.TrimLeft(value, "/")
	}
	address, err := url.Parse(value)
	if err != nil {
		return URL{}, fmt.Errorf("%w: invalid remote address %q: %w", ErrInvalidConfig, raw, err)
	}
	if address.Scheme != "gcs" {
		return URL{}, fmt.Errorf("%w: remote address %q must use the gcs:// scheme", ErrInvalidConfig, raw)
	}
	if address.Host == "" {
		return URL{}, fmt.Errorf("%w: remote address %q names no bucket", ErrInvalidConfig, raw)
	}
	address.Path = strings.TrimRight(address.Path, "/")
	return URL(*address), nil
}
