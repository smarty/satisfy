package transfer

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/smarty/gcs"
	"github.com/smarty/satisfy/v2/contracts"
	"github.com/smarty/satisfy/v2/core"
	"github.com/smarty/satisfy/v2/shell"
)

type DownloadConfig struct {
	MaxRetry          int
	Concurrency       int
	QuickVerification bool
	GoogleCredentials gcs.Credentials
	Dependencies      contracts.DependencyListing
	Logger            *slog.Logger
	Progress          contracts.ProgressFunc
}

type DownloadApp struct {
	listing     contracts.DependencyListing
	installer   *core.PackageInstaller
	integrity   contracts.IntegrityCheck
	concurrency int
	logger      *slog.Logger
}

func NewDownloadApp(config DownloadConfig) *DownloadApp {
	logger := contracts.LoggerOrDiscard(config.Logger)
	disk := shell.NewDiskFileSystem("")
	client := shell.NewGoogleCloudStorageClient(shell.NewHTTPClient(), shell.StaticCredentials(config.GoogleCredentials), []int{http.StatusPartialContent, http.StatusOK})
	retry := core.NewRetryClient(client, config.MaxRetry, core.DefaultRetryDelay, logger)
	installer := core.NewPackageInstaller(retry, disk, logger, config.Progress)
	integrity := core.NewCompoundIntegrityCheck(
		core.NewFileListingIntegrityChecker(disk),
		core.NewFileContentIntegrityCheck(md5.New, disk, !config.QuickVerification),
	)
	return &DownloadApp{
		listing:     config.Dependencies,
		installer:   installer,
		integrity:   integrity,
		concurrency: max(config.Concurrency, 1),
		logger:      logger,
	}
}

// Run installs every dependency in the listing, returning an error that joins
// the failures of each dependency that could not be installed.
func (this *DownloadApp) Run(ctx context.Context) error {
	errs := runConcurrently(ctx, this.concurrency, this.listing.Listing, func(dependency contracts.Dependency) error {
		return this.install(ctx, dependency)
	})
	if len(errs) > 0 {
		return fmt.Errorf("%d of %d packages failed to install: %w", len(errs), len(this.listing.Listing), errors.Join(errs...))
	}
	return nil
}

func (this *DownloadApp) install(ctx context.Context, dependency contracts.Dependency) error {
	resolver := core.NewDependencyResolver(shell.NewDiskFileSystem(""), this.integrity, this.installer, dependency, this.logger)
	return resolver.Resolve(ctx)
}

// runConcurrently calls work for each item, with at most limit calls in flight
// at once, and returns the errors encountered. Once ctx is done, no further
// work starts and ctx's error is among those returned.
func runConcurrently[T any](ctx context.Context, limit int, items []T, work func(T) error) (errs []error) {
	var (
		waiter    sync.WaitGroup
		lock      sync.Mutex
		semaphore = make(chan struct{}, max(limit, 1))
	)
	for _, item := range items {
		if ctx.Err() != nil {
			break
		}
		select {
		case semaphore <- struct{}{}:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
		waiter.Go(func() {
			defer func() { <-semaphore }()
			if err := work(item); err != nil {
				lock.Lock()
				errs = append(errs, err)
				lock.Unlock()
			}
		})
	}
	waiter.Wait()
	if err := ctx.Err(); err != nil {
		errs = append(errs, err)
	}
	return errs
}
