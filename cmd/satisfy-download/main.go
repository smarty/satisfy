package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"

	"bitbucket.org/smartystreets/satisfy/cmd"
	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/core"
	"bitbucket.org/smartystreets/satisfy/shell"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	config := parseConfig()
	listing := readDependencyListing(config.jsonPath)

	err := listing.Validate()
	if err != nil {
		log.Fatal(err)
	}

	working, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	disk := shell.NewDiskFileSystem(working)
	client := shell.NewGoogleCloudStorageClient(cmd.NewHTTPClient(), config.GoogleCredentials, http.StatusOK)
	installer := core.NewPackageInstaller(core.NewRetryClient(client, config.MaxRetry), disk)
	integrity := core.NewCompoundIntegrityCheck(
		core.NewFileListingIntegrityChecker(disk),
		core.NewFileContentIntegrityCheck(md5.New(), disk, config.Verify),
	)

	app := NewApp(listing, installer, integrity)
	os.Exit(app.Run())
}

func readDependencyListing(path string) (listing cmd.DependencyListing) {
	if path == "_STDIN_" {
		return readFromReader(os.Stdin)
	} else {
		return readFromFile(path)
	}
}

func readFromFile(fileName string) (listing cmd.DependencyListing) {
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
	var listing cmd.DependencyListing
	listing.Dependencies = append(listing.Dependencies, cmd.Dependency{
		Name:           "example_package_name",
		Version:        "0.0.1",
		RemoteAddress:  cmd.URL{Scheme: "gcs", Host: "bucket_name", Path: "/path/prefix"},
		LocalDirectory: "local/path",
	})
	raw, err := json.MarshalIndent(listing, "", "  ")
	if err != nil {
		log.Print(err)
	}
	log.Print("Example json file:\n", string(raw))
}

func readFromReader(reader io.Reader) (listing cmd.DependencyListing) {
	decoder := json.NewDecoder(reader)
	err := decoder.Decode(&listing)
	if err != nil {
		log.Fatal(err)
	}
	return listing
}

type App struct {
	listing   cmd.DependencyListing
	installer *core.PackageInstaller
	integrity contracts.IntegrityCheck
	waiter    *sync.WaitGroup
	results   chan error
}

func NewApp(listing cmd.DependencyListing, installer *core.PackageInstaller, integrity contracts.IntegrityCheck) *App {
	waiter := new(sync.WaitGroup)
	waiter.Add(len(listing.Dependencies))
	results := make(chan error)
	return &App{
		listing:   listing,
		installer: installer,
		integrity: integrity,
		waiter:    waiter,
		results:   results,
	}
}

func (this *App) Run() (failed int) {
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

func (this *App) awaitCompletion() {
	this.waiter.Wait()
	close(this.results)
}

func (this *App) install(dependency cmd.Dependency) {
	defer this.waiter.Done()

	log.Printf("Installing dependency: %s", dependency.Title())

	manifest, err := loadManifest(dependency)
	if err == nil && manifest.Version == dependency.Version && this.integrity.Verify(manifest) == nil {
		log.Printf("Dependency installed: %s", dependency.Title())
		return
	}
	installation := contracts.InstallationRequest{LocalPath: dependency.LocalDirectory}

	log.Printf("Downloading manifest for %s", dependency.Title())

	installation.RemoteAddress = dependency.ComposeRemoteAddress(cmd.RemoteManifestFilename)
	manifest, err = this.installer.InstallManifest(installation)
	if err != nil {
		this.results <- fmt.Errorf("failed to install manifest for %s: %v", dependency.Title(), err)
		return
	}

	log.Printf("Downloading and extracting package contents for %s", dependency.Title())

	installation.RemoteAddress = dependency.ComposeRemoteAddress(cmd.RemoteArchiveFilename)
	err = this.installer.InstallPackage(manifest, installation)
	if err != nil {
		this.results <- fmt.Errorf("failed to install package contents for %s: %v", dependency.Title(), err)
		return
	}

	log.Printf("Dependency installed: %s", dependency.Title())
}

func loadManifest(dependency cmd.Dependency) (manifest contracts.Manifest, err error) {
	path := core.ComposeManifestPath(dependency.LocalDirectory, dependency.Name)

	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return manifest, errNotInstalled
	}

	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return manifest, err
	}

	err = json.Unmarshal(raw, &manifest)
	if err != nil {
		return manifest, err
	}

	return manifest, nil
}

var (
	errNotInstalled = errors.New("package not yet installed")
)
