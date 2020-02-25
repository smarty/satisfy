package main

import (
	"log"
	"net/http"
	"strings"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/core"
	"bitbucket.org/smartystreets/satisfy/shell"
)

type CheckApp struct {
	config contracts.UploadConfig
	client contracts.RemoteStorage
}

func NewCheckApp(config contracts.UploadConfig) *CheckApp {
	return &CheckApp{config: config}
}

func (this *CheckApp) Run() {
	if this.config.Overwrite {
		log.Println("[INFO] Overwrite mode enabled, skipping remote manifest check.")
		return
	}
	if returnCode, success := this.sanityCheck(contracts.RemoteManifestFilename) ; !success {
		log.Fatal("[INFO] Sanity check failed.", returnCode)
	}
}

func (this *CheckApp) sanityCheck(path string) (string, bool) {
	this.buildRemoteStorageClient()

	_, err := this.client.Download(this.config.PackageConfig.ComposeRemoteAddress(path))
	// TODO: inspect this error: if the response is HTTP 200 (file exists), exit with return code 1; if general failure, exit with return code 2
	if err != nil {
		return gatherReturnCode(err), false
	}
	return "", true
}

func gatherReturnCode(err error) string {
	if strings.Contains(err.Error(), "file exists") {
		return "return code 1"
	}

	return "return code 2"
}

func (this *CheckApp) buildRemoteStorageClient() {
	client := shell.NewHTTPClient()
	gcsClient := shell.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, http.StatusNotFound)
	this.client = core.NewRetryClient(gcsClient, this.config.MaxRetry)
}
