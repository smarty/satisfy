package main

import (
	"crypto/md5"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/core"
	"bitbucket.org/smartystreets/satisfy/shell"
)

func readDependencyListing(path string) (listing contracts.DependencyListing) {
	if path == "_STDIN_" {
		return readFromReader(os.Stdin)
	} else {
		return readFromFile(path)
	}
}

func readFromFile(fileName string) (listing contracts.DependencyListing) {
	file, err := os.Open(fileName)
	if os.IsNotExist(err) {
		emitExampleDependenciesFile()
		log.Fatalln("Specified dependency file not found:", fileName)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	return readFromReader(file)
}

func emitExampleDependenciesFile() {
	var listing contracts.DependencyListing
	listing.Dependencies = append(listing.Dependencies, contracts.Dependency{
		PackageName:    "example_package_name",
		PackageVersion: "0.0.1",
		RemoteAddress:  contracts.URL{Scheme: "gcs", Host: "bucket_name", Path: "/path/prefix"},
		LocalDirectory: "local/path",
	})
	raw, err := json.MarshalIndent(listing, "", "  ")
	if err != nil {
		log.Print(err)
	}
	log.Print("Example json file:\n", string(raw))
}

func readFromReader(reader io.Reader) (listing contracts.DependencyListing) {
	decoder := json.NewDecoder(reader)
	err := decoder.Decode(&listing)
	if err != nil {
		log.Fatal(err)
	}
	return listing
}

type DownloadApp struct {
	listing   contracts.DependencyListing
	installer *core.PackageInstaller
	integrity contracts.IntegrityCheck
	waiter    *sync.WaitGroup
	results   chan error
}

func NewDownloadApp(config DownloadConfig) *DownloadApp {
	listing := readDependencyListing(config.JSONPath)

	err := listing.Validate()
	if err != nil {
		log.Fatal(err)
	}

	listing.Dependencies = core.Filter(listing.Dependencies, config.packageFilter)

	if len(listing.Dependencies) == 0 {
		log.Println("[WARN] No dependencies provided. You can go about your business. Move along.")
		emitExampleDependenciesFile()
	}

	disk := shell.NewDiskFileSystem("")
	client := shell.NewGoogleCloudStorageClient(shell.NewHTTPClient(), config.GoogleCredentials, http.StatusOK)
	installer := core.NewPackageInstaller(core.NewRetryClient(client, config.MaxRetry), disk)
	integrity := core.NewCompoundIntegrityCheck(
		core.NewFileListingIntegrityChecker(disk),
		core.NewFileContentIntegrityCheck(md5.New, disk, !config.QuickVerification),
	)
	waiter := new(sync.WaitGroup)
	waiter.Add(len(listing.Dependencies))
	results := make(chan error)
	return &DownloadApp{
		listing:   listing,
		installer: installer,
		integrity: integrity,
		waiter:    waiter,
		results:   results,
	}
}

func (this *DownloadApp) Run() (failed int) {
	for _, dependency := range this.listing.Dependencies {
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
