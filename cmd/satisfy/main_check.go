package main

import (
	"log"
	"net/http"

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

func (this *CheckApp) Run() int {
	if this.config.Overwrite {
		log.Println("[INFO] Overwrite mode enabled, skipping remote manifest check.")
		return 0
	}

	client := this.buildRemoteStorageClient()
	address := this.config.PackageConfig.ComposeRemoteAddress(contracts.RemoteManifestFilename)
	_, err := client.Download(address)
	if err == nil {
		return 0
	}

	statusError, ok := err.(*contracts.StatusCodeError)
	if ok && statusError.StatusCode() == http.StatusOK {
		log.Println("[INFO] Package already exists on remote storage.")
		return 1
	}

	log.Println("[WARN] Sanity check failed:", err)
	return 2
}

func (this *CheckApp) buildRemoteStorageClient() contracts.Downloader {
	client := shell.NewHTTPClient()
	gcsClient := shell.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, http.StatusNotFound)
	return core.NewRetryClient(gcsClient, this.config.MaxRetry)
}
