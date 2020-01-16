package main

import (
	"log"
	"net/http"
	"time"

	"bitbucket.org/smartystreets/satisfy/cmd"
	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/remote"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	NewApp(cmd.ParseConfig()).Run()
	log.Println("OK")
}

type App struct {
	config cmd.Config
	client contracts.RemoteStorage
}

func NewApp(config cmd.Config) *App {
	return &App{config: config}
}

func (this *App) Run() {
	this.buildRemoteStorageClient()

	if this.uploadedPreviously() {
		log.Fatal("[INFO] Package manifest already present on remote storage. You can go about your business. Move along.")
	}
}

func (this *App) uploadedPreviously() bool {
	request := contracts.DownloadRequest{
		Bucket:   this.config.RemoteBucket,
		Resource: this.config.ComposeRemotePath(cmd.RemoteManifestFilename),
	}
	reader, err := this.client.Download(request)
	if err != nil {
		return false
	}
	_ = reader.Close()
	return true
}

func (this *App) buildRemoteStorageClient() {
	client := &http.Client{Timeout: time.Minute}
	gcsClient := remote.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials)
	this.client = remote.NewRetryClient(gcsClient, this.config.MaxRetry)
}
