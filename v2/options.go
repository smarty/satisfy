package satisfy

import (
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/smarty/gcs"
	"github.com/smarty/satisfy/v2/internal/model"
	"github.com/smarty/satisfy/v2/internal/storage"
)

// Option configures a Client.
type Option func(*clientConfig)

type clientConfig struct {
	httpClient     *http.Client
	credentials    storage.CredentialsSource
	maxRetry       int
	retryDelay     time.Duration
	concurrency    int
	verifyContents bool
	logger         *slog.Logger
	progress       model.ProgressFunc
	storage        model.RemoteStorage // for tests: replaces the GCS client
	tempDirectory  string              // for tests: where archives are built ("" = os.TempDir)
}

func defaultClientConfig() clientConfig {
	return clientConfig{
		httpClient:  storage.NewHTTPClient(),
		credentials: storage.NewRefreshingCredentials(gcs.NewCredentialsReader(), "", storage.DefaultCredentialsMaxAge),
		maxRetry:    5,
		retryDelay:  storage.DefaultRetryDelay,
		concurrency: runtime.NumCPU(),
	}
}

// WithHTTPClient sets the HTTP client used to reach Google Cloud Storage.
func WithHTTPClient(client *http.Client) Option {
	return func(this *clientConfig) {
		if client != nil {
			this.httpClient = client
		}
	}
}

// WithCredentials uses fixed credentials for every request. Bearer tokens
// expire, so prefer WithCredentialsReader for long-running work.
func WithCredentials(credentials gcs.Credentials) Option {
	return func(this *clientConfig) { this.credentials = storage.StaticCredentials(credentials) }
}

// WithCredentialsReader reads credentials from reader on first use and again
// whenever they are 30 minutes old. Without this option or WithCredentials,
// credentials are discovered from the environment by gcs.NewCredentialsReader.
func WithCredentialsReader(reader gcs.CredentialsReader) Option {
	return func(this *clientConfig) {
		this.credentials = storage.NewRefreshingCredentials(reader, "", storage.DefaultCredentialsMaxAge)
	}
}

// WithMaxRetry sets how many times a failed request is retried (default 5).
func WithMaxRetry(maxRetry int) Option {
	return func(this *clientConfig) { this.maxRetry = max(maxRetry, 0) }
}

// WithRetryDelay sets the pause between retries (default 3 seconds).
func WithRetryDelay(delay time.Duration) Option {
	return func(this *clientConfig) { this.retryDelay = max(delay, 0) }
}

// WithConcurrency sets how many packages Install handles at once (default:
// the number of CPUs).
func WithConcurrency(concurrency int) Option {
	return func(this *clientConfig) { this.concurrency = max(concurrency, 1) }
}

// WithContentVerification makes Install verify the checksum of every
// installed file, not just its presence and size, before deciding a package
// is already installed (default false).
func WithContentVerification(enabled bool) Option {
	return func(this *clientConfig) { this.verifyContents = enabled }
}

// WithLogger sends log records to logger. By default nothing is logged.
func WithLogger(logger *slog.Logger) Option {
	return func(this *clientConfig) { this.logger = logger }
}

// WithProgress receives byte-level progress while archiving, downloading and
// extracting. The callback may be called from several goroutines at once.
func WithProgress(progress func(Progress)) Option {
	return func(this *clientConfig) {
		this.progress = nil
		if progress != nil {
			this.progress = func(report model.Progress) { progress(publicProgress(report)) }
		}
	}
}

func withStorage(storage model.RemoteStorage) Option {
	return func(this *clientConfig) { this.storage = storage }
}

func withTempDirectory(directory string) Option {
	return func(this *clientConfig) { this.tempDirectory = directory }
}
