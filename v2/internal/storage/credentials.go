package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/smarty/gcs"
	"github.com/smarty/satisfy/v2/internal/model"
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
// bearer token.
//
// When reading them again fails, the current credentials are still used
// while they are younger than maxAge*3/2 (45 minutes by default, well within
// a one hour token lifetime), so a transient failure of the credential source
// costs nothing. Past that, or once Invalidate has discarded them, the
// failure is returned, marked retryable if credentials were read before. It
// is safe for concurrent use.
type RefreshingCredentials struct {
	reader gcs.CredentialsReader
	value  string
	maxAge time.Duration
	now    func() time.Time

	lock     sync.Mutex
	current  gcs.Credentials
	readAt   time.Time
	loaded   bool
	readOnce bool // a read has succeeded, so the source is configured
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
		if this.loaded && now.Sub(this.readAt) < this.maxAge*3/2 {
			return this.current, nil
		}
		if this.readOnce {
			return gcs.Credentials{}, fmt.Errorf("%w (%w)", err, model.ErrRetryable)
		}
		return gcs.Credentials{}, err
	}
	this.current, this.readAt, this.loaded, this.readOnce = credentials, now, true, true
	return credentials, nil
}

// Invalidate discards the current credentials, so that the next request reads
// them again (for example, after the server rejected them as expired).
func (this *RefreshingCredentials) Invalidate() {
	this.lock.Lock()
	defer this.lock.Unlock()
	this.current, this.loaded = gcs.Credentials{}, false
}
