package main

import (
	"log"
	"net/http"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/core"
	"bitbucket.org/smartystreets/satisfy/shell"
)

type CheckApp struct {
	config UploadConfig
	client contracts.RemoteStorage
}

func NewCheckApp(config UploadConfig) *CheckApp {
	return &CheckApp{config: config}
}

func (this *CheckApp) Run() {
	if this.config.Overwrite {
		log.Println("[INFO] Overwrite mode enabled, skipping remote manifest check.")
		return
	}
	if this.uploadedPreviously(contracts.RemoteManifestFilename) {
		log.Fatal("[INFO] Package manifest already present on remote storage. You can go about your business. Move along.")
	}
}

func (this *CheckApp) uploadedPreviously(path string) bool {
	this.buildRemoteStorageClient()

	_, err := this.client.Download(this.config.PackageConfig.ComposeRemoteAddress(path))
	return err != nil
}

func (this *CheckApp) buildRemoteStorageClient() {
	client := shell.NewHTTPClient()
	gcsClient := shell.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, http.StatusNotFound)
	this.client = core.NewRetryClient(gcsClient, this.config.MaxRetry)
}
