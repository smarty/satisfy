package transfer

import (
	"fmt"
	"net/http"
	"time"

	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/internal/core"
	"github.com/smarty/satisfy/internal/plumbing"
	"github.com/smarty/satisfy/internal/shell"
)

type CheckApp struct {
	config contracts.CheckConfiguration
}

func NewCheckApp(config contracts.CheckConfiguration) *CheckApp {
	return &CheckApp{config: config}
}

func (this *CheckApp) Run(yield func(contracts.Event, error) bool) {
	if this.config.Overwrite {
		yield(contracts.Event{Type: contracts.EventInfo, Message: "Overwrite mode enabled, skipping remote manifest check."}, nil)
		return
	}

	address := this.config.PackageConfig.ComposeRemoteAddress(contracts.RemoteManifestFilename)
	body, err := this.buildRemoteStorageClient().Download(address)
	if err == nil {
		_ = body.Close()
		return
	}

	if code, ok := contracts.StatusCode(err); ok && code == http.StatusOK {
		yield(contracts.Event{}, contracts.ErrPackageExists)
		return
	}

	yield(contracts.Event{}, fmt.Errorf("sanity check failed: %w", err))
}

func (this *CheckApp) buildRemoteStorageClient() plumbing.Downloader {
	client := shell.NewHTTPClient()
	gcsClient := shell.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, []int{http.StatusNotFound})
	return core.NewRetryClient(gcsClient, this.config.MaxRetry, time.Sleep, nil)
}
