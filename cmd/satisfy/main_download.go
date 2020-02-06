package main

import (
	"crypto/md5"
	"log"
	"net/http"
	"sync"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/core"
	"bitbucket.org/smartystreets/satisfy/shell"
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
	installer := core.NewPackageInstaller(core.NewRetryClient(client, config.MaxRetry), disk)
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

func (this *DownloadApp) Run() (failed int) {
	for _, dependency := range this.listing.Listing {
		go this.install(dependency)
	}
	go this.awaitCompletion()
	for err := range this.results {
		failed++
		log.Println("[WARN]", err)
	}
	return failed
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
