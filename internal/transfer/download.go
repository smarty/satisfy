package transfer

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/internal/core"
	"github.com/smarty/satisfy/internal/shell"
)

type DownloadApp struct {
	config  contracts.DownloadConfiguration
	listing contracts.DependencyListing
}

func NewDownloadApp(config contracts.DownloadConfiguration) *DownloadApp {
	return &DownloadApp{
		config:  config,
		listing: config.Dependencies,
	}
}

func (this *DownloadApp) Run(yield func(contracts.Event, error) bool) {
	orch := newDownloadOrchestrator(len(this.listing.Listing))

	disk := shell.NewDiskFileSystem("")
	client := shell.NewGoogleCloudStorageClient(shell.NewHTTPClient(), this.config.GoogleCredentials, []int{http.StatusPartialContent, http.StatusOK})
	downloader := core.NewRetryClient(client, this.config.MaxRetry, time.Sleep, orch.emitEvent)
	integrity := core.NewCompoundIntegrityCheck(
		core.NewFileListingIntegrityChecker(disk, orch.emitEvent),
		core.NewFileContentIntegrityCheck(md5.New, disk, !this.config.QuickVerification, orch.emitEvent),
	)
	installer := core.NewPackageInstaller(downloader, disk, this.config.NewProgress, orch.emitEvent)

	waiter := new(sync.WaitGroup)
	waiter.Add(len(this.listing.Listing))

	for _, dep := range this.listing.Listing {
		go func() {
			defer waiter.Done()
			resolver := core.NewDependencyResolver(shell.NewDiskFileSystem(""), integrity, installer, dep, orch.emitEvent)
			if err := resolver.Resolve(); err != nil {
				orch.emitError(err)
			}
		}()
	}

	go func() {
		waiter.Wait()
		close(orch.results)
		close(orch.events)
	}()

	results := orch.results
	events := orch.events
	failed := 0

	for results != nil || events != nil {
		select {
		case err, ok := <-results:
			if !ok {
				results = nil
				continue
			}

			failed++
			if !yield(contracts.Event{Type: contracts.EventFailure, Message: err.Error()}, nil) {
				orch.cancel()
				return
			}

		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}

			if !yield(event, nil) {
				orch.cancel()
				return
			}
		}
	}

	if failed > 0 {
		yield(contracts.Event{}, fmt.Errorf("%d package(s) failed to install", failed))
	}
}
