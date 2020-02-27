package main

import (
	"log"
	"net/http"
	"os"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/core"
	"bitbucket.org/smartystreets/satisfy/shell"
)

type CheckApp struct {
	config contracts.UploadConfig
}

func NewCheckApp(config contracts.UploadConfig) *CheckApp {
	return &CheckApp{config: config}
}

func (this *CheckApp) Run() {
	if this.config.Overwrite {
		log.Println("[INFO] Overwrite mode enabled, skipping remote manifest check.")
		return
	}

	client := this.buildRemoteStorageClient()
	address := this.config.PackageConfig.ComposeRemoteAddress(contracts.RemoteManifestFilename)
	_, err := client.Download(address)
	if err == nil {
		return
	}

	statusError, ok := err.(*contracts.StatusCodeError)
	if ok && statusError.StatusCode() == http.StatusOK {
		log.Println("[INFO] Package already exists on remote storage.")
		os.Exit(2)
	}

	log.Fatalln("[WARN] Sanity check failed:", err)
}

func (this *CheckApp) buildRemoteStorageClient() contracts.Downloader {
	client := shell.NewHTTPClient()
	gcsClient := shell.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, http.StatusNotFound)
	return core.NewRetryClient(gcsClient, this.config.MaxRetry)
}
