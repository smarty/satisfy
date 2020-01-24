package main

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/klauspost/compress/zstd"

	"bitbucket.org/smartystreets/satisfy/cmd"
	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/core"
	"bitbucket.org/smartystreets/satisfy/shell"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	if len(os.Args) == 1 {
		os.Args = append(os.Args, "")
	}
	switch os.Args[1] {
	case "upload":
		uploadMain(os.Args[2:])
	case "check":
		checkMain(os.Args[2:])
	default:
		downloadMain()
	}
}

func checkMain(args []string) {
	NewCheckApp(cmd.ParseConfig(args)).Run()
}

func uploadMain(args []string) {
	NewUploadApp(cmd.ParseConfig(args)).Run()
}

func downloadMain() {
	config := parseConfig()
	listing := readDependencyListing(config.JSONPath)

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
		core.NewFileContentIntegrityCheck(md5.New, disk, !config.QuickVerification),
	)

	app := NewDownloadApp(listing, installer, integrity)
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

type DownloadApp struct {
	listing   cmd.DependencyListing
	installer *core.PackageInstaller
	integrity contracts.IntegrityCheck
	waiter    *sync.WaitGroup
	results   chan error
}

func NewDownloadApp(listing cmd.DependencyListing, installer *core.PackageInstaller, integrity contracts.IntegrityCheck) *DownloadApp {
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

func (this *DownloadApp) install(dependency cmd.Dependency) {
	defer this.waiter.Done()

	log.Printf("Installing dependency: %s", dependency.Title())

	manifest, manifestErr := loadManifest(dependency)
	if manifestErr == nil && manifest.Version == dependency.Version {
		verifyErr := this.integrity.Verify(manifest, dependency.LocalDirectory)
		if verifyErr == nil {
			log.Printf("Dependency already installed: %s", dependency.Title())
			return
		} else {
			log.Printf("%s in %s", verifyErr.Error(), dependency.Title())
		}
	}
	installation := contracts.InstallationRequest{LocalPath: dependency.LocalDirectory}

	log.Printf("Downloading manifest for %s", dependency.Title())

	installation.RemoteAddress = dependency.ComposeRemoteAddress(cmd.RemoteManifestFilename)
	manifest, err := this.installer.InstallManifest(installation)
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

//////////////////////////////////////////////////////////////

type UploadApp struct {
	config     cmd.Config
	file       *os.File
	hasher     hash.Hash
	compressor io.WriteCloser
	builder    *core.PackageBuilder
	manifest   contracts.Manifest
	client     contracts.RemoteStorage
}

func NewUploadApp(config cmd.Config) *UploadApp {
	return &UploadApp{config: config}
}

func (this *UploadApp) Run() {
	this.buildRemoteStorageClient()

	log.Println("Building the archive...")
	this.buildArchiveAndManifestContents()
	this.completeManifest()

	log.Println("Manifest:", this.dumpManifest())

	log.Println("Uploading the archive...")
	this.upload(this.buildArchiveUploadRequest())
	this.closeArchiveFile()
	this.deleteLocalArchiveFile()

	log.Println("Uploading the manifest...")
	this.upload(this.buildManifestUploadRequest())
}

func (this *UploadApp) buildArchiveUploadRequest() contracts.UploadRequest {
	this.openArchiveFile()
	return contracts.UploadRequest{
		RemoteAddress: this.config.ComposeRemoteAddress(cmd.RemoteArchiveFilename),
		Body:          NewFileWrapper(this.file),
		Size:          int64(this.manifest.Archive.Size),
		ContentType:   "application/zstd",
		Checksum:      this.manifest.Archive.MD5Checksum,
	}
}

func (this *UploadApp) buildArchiveAndManifestContents() {
	var err error
	this.file, err = ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}
	this.hasher = md5.New()
	writer := io.MultiWriter(this.hasher, this.file)
	this.InitializeCompressor(writer)

	this.builder = core.NewPackageBuilder(
		shell.NewDiskFileSystem(this.config.SourceDirectory),
		shell.NewTarArchiveWriter(this.compressor),
		md5.New(),
	)

	err = this.builder.Build()
	if err != nil {
		log.Fatal(err)
	}

	err = this.compressor.Close()
	if err != nil {
		log.Fatal(err)
	}

	this.closeArchiveFile()
}

func (this *UploadApp) InitializeCompressor(writer io.Writer) {
	factory, found := compression[this.config.CompressionAlgorithm]
	if !found {
		log.Fatalln("Unsupported compression algorithm:", this.config.CompressionAlgorithm)
	}
	this.compressor = factory(writer, this.config.CompressionLevel)
}

var compression = map[string]func(_ io.Writer, level int) io.WriteCloser{
	"zstd": func(writer io.Writer, level int) io.WriteCloser {
		compressor, err := zstd.NewWriter(writer, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)))
		if err != nil {
			log.Fatal(err)
		}
		return compressor
	},
	"gzip": func(writer io.Writer, level int) io.WriteCloser {
		compressor, err := gzip.NewWriterLevel(writer, level)
		if err != nil {
			log.Panicln(err)
		}
		return compressor
	},
}

func (this *UploadApp) buildManifestUploadRequest() contracts.UploadRequest {
	buffer := this.writeManifestToBuffer()
	return contracts.UploadRequest{
		RemoteAddress: this.config.ComposeRemoteAddress(cmd.RemoteManifestFilename),
		Body:          bytes.NewReader(buffer.Bytes()),
		Size:          int64(buffer.Len()),
		ContentType:   "application/json",
		Checksum:      this.hasher.Sum(nil),
	}
}

func (this *UploadApp) buildRemoteStorageClient() {
	client := cmd.NewHTTPClient()
	gcsClient := shell.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, http.StatusOK)
	this.client = core.NewRetryClient(gcsClient, this.config.MaxRetry)
}

func (this *UploadApp) completeManifest() {
	fileInfo, err := os.Stat(this.file.Name())
	if err != nil {
		log.Fatal(err)
	}
	this.manifest = contracts.Manifest{
		Name:    this.config.PackageName,
		Version: this.config.PackageVersion,
		Archive: contracts.Archive{
			Filename:             cmd.RemoteArchiveFilename,
			Size:                 uint64(fileInfo.Size()),
			MD5Checksum:          this.hasher.Sum(nil),
			Contents:             this.builder.Contents(),
			CompressionAlgorithm: this.config.CompressionAlgorithm,
		},
	}
}

func (this *UploadApp) closeArchiveFile() {
	err := this.file.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func (this *UploadApp) deleteLocalArchiveFile() {
	err := os.Remove(this.file.Name())
	if err != nil {
		log.Fatal(err)
	}
}

func (this *UploadApp) openArchiveFile() {
	var err error
	this.file, err = os.Open(this.file.Name())
	if err != nil {
		log.Fatal(err)
	}
}

func (this *UploadApp) upload(request contracts.UploadRequest) {
	err := this.client.Upload(request)
	if err != nil {
		log.Fatal(err)
	}
}

func (this *UploadApp) writeManifestToBuffer() *bytes.Buffer {
	buffer := new(bytes.Buffer)
	this.hasher.Reset()
	writer := io.MultiWriter(buffer, this.hasher)
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(this.manifest)
	return buffer
}

func (this *UploadApp) dumpManifest() string {
	raw, err := json.MarshalIndent(this.manifest, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	return "\n"+string(raw)
}

///////////////////////////////////////////////////////////////

type CheckApp struct {
	config cmd.Config
	client contracts.RemoteStorage
}

func NewCheckApp(config cmd.Config) *CheckApp {
	return &CheckApp{config: config}
}

func (this *CheckApp) Run() {
	if this.uploadedPreviously(cmd.RemoteManifestFilename) {
		log.Fatal("[INFO] Package manifest already present on remote storage. You can go about your business. Move along.")
	}
}

func (this *CheckApp) uploadedPreviously(path string) bool {
	this.buildRemoteStorageClient()

	_, err := this.client.Download(this.config.ComposeRemoteAddress(path))
	return err != nil
}

func (this *CheckApp) buildRemoteStorageClient() {
	client := cmd.NewHTTPClient()
	gcsClient := shell.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, http.StatusNotFound)
	this.client = core.NewRetryClient(gcsClient, this.config.MaxRetry)
}
