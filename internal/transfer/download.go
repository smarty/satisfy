package transfer

import (
	"crypto/md5"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/smarty/satisfy/configuration"
	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/internal/core"
	"github.com/smarty/satisfy/internal/shell"
)

type DownloadApp struct {
	listing   configuration.DependencyListing
	installer *core.PackageInstaller
	integrity contracts.IntegrityCheck
	waiter    *sync.WaitGroup
	results   chan error
}

func NewDownloadApp(config configuration.DownloadConfiguration) *DownloadApp {
	disk := shell.NewDiskFileSystem("")
	client := shell.NewGoogleCloudStorageClient(shell.NewHTTPClient(), config.GoogleCredentials, []int{http.StatusPartialContent, http.StatusOK})
	installer := core.NewPackageInstaller(core.NewRetryClient(client, config.MaxRetry, time.Sleep), disk, config.NewProgress)
	integrity := core.NewCompoundIntegrityCheck(
		core.NewFileListingIntegrityChecker(disk),
		core.NewFileContentIntegrityCheck(md5.New, disk, !config.QuickVerification),
	)
	waiter := new(sync.WaitGroup)
	waiter.Add(len(config.Dependencies.Listing))
	return &DownloadApp{
		listing:   config.Dependencies,
		installer: installer,
		integrity: integrity,
		waiter:    waiter,
		results:   make(chan error),
	}
}

func (this *DownloadApp) Run() {
	err := this.TryRun()
	if err != nil {
		log.Fatal(err)
	}
}

func (this *DownloadApp) TryRun() error {
	for _, dependency := range this.listing.Listing {
		go this.install(dependency)
	}
	go this.awaitCompletion()
	failed := 0
	for err := range this.results {
		failed++
		log.Println("[WARN]", err)
	}
	if failed > 0 {
		return fmt.Errorf("[WARN] %d packages failed to install.", failed)
	}
	return nil
}

func (this *DownloadApp) awaitCompletion() {
	this.waiter.Wait()
	close(this.results)
}

func (this *DownloadApp) install(dependency configuration.Dependency) {
	defer this.waiter.Done()

	resolver := core.NewDependencyResolver(shell.NewDiskFileSystem(""), this.integrity, this.installer, dependency)
	err := resolver.Resolve()
	if err != nil {
		this.results <- err
	}
}
