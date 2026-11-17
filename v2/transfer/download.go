package transfer

import (
	"crypto/md5"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/smarty/satisfy/v2/contracts"
	"github.com/smarty/satisfy/v2/core"
	"github.com/smarty/satisfy/v2/shell"
)

type DownloadApp struct {
	listing     contracts.DependencyListing
	installer   *core.PackageInstaller
	integrity   contracts.IntegrityCheck
	concurrency int
}

func NewDownloadApp(config DownloadConfig) *DownloadApp {
	disk := shell.NewDiskFileSystem("")
	client := shell.NewGoogleCloudStorageClient(shell.NewHTTPClient(), config.GoogleCredentials, []int{http.StatusPartialContent, http.StatusOK})
	installer := core.NewPackageInstaller(core.NewRetryClient(client, config.MaxRetry, time.Sleep), disk, config.ShowProgress)
	integrity := core.NewCompoundIntegrityCheck(
		core.NewFileListingIntegrityChecker(disk),
		core.NewFileContentIntegrityCheck(md5.New, disk, !config.QuickVerification),
	)
	return &DownloadApp{
		listing:     config.Dependencies,
		installer:   installer,
		integrity:   integrity,
		concurrency: max(config.Concurrency, 1),
	}
}

func (this *DownloadApp) Run() error {
	errs := runConcurrently(this.concurrency, this.listing.Listing, this.install)
	for _, err := range errs {
		log.Println("[WARN]", err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("[WARN] %d packages failed to install.", len(errs))
	}
	return nil
}

func (this *DownloadApp) install(dependency contracts.Dependency) error {
	resolver := core.NewDependencyResolver(shell.NewDiskFileSystem(""), this.integrity, this.installer, dependency)
	return resolver.Resolve()
}

// runConcurrently calls work for each item, with at most limit calls in flight
// at once, and returns the errors encountered.
func runConcurrently[T any](limit int, items []T, work func(T) error) (errs []error) {
	var (
		waiter    sync.WaitGroup
		lock      sync.Mutex
		semaphore = make(chan struct{}, max(limit, 1))
	)
	for _, item := range items {
		semaphore <- struct{}{}
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
	return errs
}
