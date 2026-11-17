package transfer

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/smarty/satisfy/v2/contracts"
	"github.com/smarty/satisfy/v2/core"
	"github.com/smarty/satisfy/v2/shell"
)

type CheckApp struct {
	config contracts.UploadConfig
	client contracts.Downloader
	logger *slog.Logger
}

func NewCheckApp(config contracts.UploadConfig) *CheckApp {
	logger := contracts.LoggerOrDiscard(config.Logger)
	gcsClient := shell.NewGoogleCloudStorageClient(shell.NewHTTPClient(), shell.StaticCredentials(config.GoogleCredentials), []int{http.StatusOK})
	return &CheckApp{
		config: config,
		client: core.NewRetryClient(gcsClient, config.MaxRetry, core.DefaultRetryDelay, logger),
		logger: logger,
	}
}

// Run returns contracts.ErrPackageExists when the package version described by
// the configuration is already present on remote storage (unless overwriting).
func (this *CheckApp) Run(ctx context.Context) error {
	if this.config.Overwrite {
		this.logger.Info("overwrite mode enabled; skipping remote manifest check")
		return nil
	}

	address := this.config.PackageConfig.ComposeRemoteAddress(contracts.RemoteManifestFilename)
	_, err := this.client.Size(ctx, address)
	if contracts.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("sanity check failed: %w", err)
	}
	return contracts.ErrPackageExists
}
