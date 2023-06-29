package satisfy

import (
	"crypto/md5"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/smartystreets/satisfy/contracts"
	"github.com/smartystreets/satisfy/core"
	"github.com/smartystreets/satisfy/shell"
)

type DownloadApp struct {
	listing   contracts.DependencyListing
	installer *core.PackageInstaller
	integrity contracts.IntegrityCheck
	waiter    *sync.WaitGroup
	results   chan error
}

func NewDownloadApp(config DownloadConfig) *DownloadApp {
	disk := shell.NewDiskFileSystem("")
	client := shell.NewGoogleCloudStorageClient(shell.NewHTTPClient(), config.GoogleCredentials, http.StatusOK)
	installer := core.NewPackageInstaller(core.NewRetryClient(client, config.MaxRetry, time.Sleep), disk)
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
		log.Fatalf("[WARN] %d packages failed to install.", failed)
	}
}

func (this *DownloadApp) awaitCompletion() {
	this.waiter.Wait()
	close(this.results)
}

func (this *DownloadApp) install(dependency contracts.Dependency) {
	defer this.waiter.Done()

	resolver := core.NewDependencyResolver(shell.NewDiskFileSystem(""), this.integrity, this.installer, dependency)
	err := resolver.Resolve()
	if err != nil {
		this.results <- err
	}
}
