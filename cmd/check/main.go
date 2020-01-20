package main

import (
	"log"
	"net/http"

	"bitbucket.org/smartystreets/satisfy/cmd"
	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/core"
	"bitbucket.org/smartystreets/satisfy/shell"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	NewApp(cmd.ParseConfig()).Run()
}

type App struct {
	config cmd.Config
	client contracts.RemoteStorage
}

func NewApp(config cmd.Config) *App {
	return &App{config: config}
}

func (this *App) Run() {
	if this.uploadedPreviously(cmd.RemoteManifestFilename) {
		log.Fatal("[INFO] Package manifest already present on remote storage. You can go about your business. Move along.")
	}
}

func (this *App) uploadedPreviously(path string) bool {
	this.buildRemoteStorageClient()

	_, err := this.client.Download(this.config.ComposeRemoteAddress(path))
	return err != nil
}

func (this *App) buildRemoteStorageClient() {
	client := cmd.NewHTTPClient()
	gcsClient := shell.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, http.StatusNotFound)
	this.client = core.NewRetryClient(gcsClient, this.config.MaxRetry)
}
