package storage

import (
	"context"
	"sync"
	"time"

	"github.com/smarty/gcs"
)

// CredentialsSource supplies the credentials used to sign or authorize each
// request to Google Cloud Storage.
type CredentialsSource interface {
	Credentials(context.Context) (gcs.Credentials, error)
}

// StaticCredentials always supplies the same credentials.
type StaticCredentials gcs.Credentials

func (this StaticCredentials) Credentials(context.Context) (gcs.Credentials, error) {
	return gcs.Credentials(this), nil
}

// RefreshingCredentials reads credentials on first use and reads them again
// once they are older than maxAge, so that long-running operations (such as
// compressing a large archive before uploading it) never use an expired
// bearer token. It is safe for concurrent use.
type RefreshingCredentials struct {
	reader gcs.CredentialsReader
	value  string
	maxAge time.Duration
	now    func() time.Time

	lock    sync.Mutex
	current gcs.Credentials
	readAt  time.Time
	loaded  bool
}

// DefaultCredentialsMaxAge is comfortably shorter than the one hour lifetime
// of Google OAuth access tokens.
const DefaultCredentialsMaxAge = 30 * time.Minute

// NewRefreshingCredentials returns a source backed by reader. The value is
// passed to reader.Read and may hold an explicit access token (or be empty).
func NewRefreshingCredentials(reader gcs.CredentialsReader, value string, maxAge time.Duration) *RefreshingCredentials {
	if maxAge <= 0 {
		maxAge = DefaultCredentialsMaxAge
	}
	return &RefreshingCredentials{reader: reader, value: value, maxAge: maxAge, now: time.Now}
}

func (this *RefreshingCredentials) Credentials(ctx context.Context) (gcs.Credentials, error) {
	this.lock.Lock()
	defer this.lock.Unlock()

	now := this.now()
	if this.loaded && now.Sub(this.readAt) < this.maxAge {
		return this.current, nil
	}
	credentials, err := this.reader.Read(ctx, this.value)
	if err != nil {
		return gcs.Credentials{}, err
	}
	this.current, this.readAt, this.loaded = credentials, now, true
	return credentials, nil
}
