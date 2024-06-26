package transfer

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/core"
	"github.com/smarty/satisfy/shell"
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
	body, err := client.Download(address)
	if err == nil {
		defer func() { _ = body.Close() }()
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
	gcsClient := shell.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, []int{http.StatusNotFound})
	return core.NewRetryClient(gcsClient, this.config.MaxRetry, time.Sleep)
}
