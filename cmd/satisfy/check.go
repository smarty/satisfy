package main

import (
	"log"
	"net/http"

	"bitbucket.org/smartystreets/satisfy/cmd"
	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/core"
	"bitbucket.org/smartystreets/satisfy/shell"
)

func checkMain(args []string) {
	NewCheckApp(cmd.ParseConfig(args)).Run()
}

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