package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/smarty/gcs"

	"github.com/smarty/satisfy/v2/contracts"
	"github.com/smarty/satisfy/v2/core"
	"github.com/smarty/satisfy/v2/shell"
)

type LatestConfig struct {
	PackageName       string
	RemoteAddress     contracts.URL
	GoogleCredentials gcs.Credentials
	MaxRetry          int
	Logger            *slog.Logger
}

type LatestApp struct {
	config LatestConfig
}

func NewLatestApp(config LatestConfig) *LatestApp {
	return &LatestApp{config: config}
}

// Run returns the version recorded in the package's root (latest) manifest.
func (this *LatestApp) Run(ctx context.Context) (string, error) {
	manifest, err := downloadRootManifest(ctx, this.config.GoogleCredentials, this.config.MaxRetry, this.config.Logger,
		this.config.RemoteAddress, this.config.PackageName)
	if err != nil {
		return "", err
	}
	return manifest.Version, nil
}

func downloadRootManifest(
	ctx context.Context,
	credentials gcs.Credentials,
	maxRetry int,
	logger *slog.Logger,
	remoteAddress contracts.URL,
	packageName string,
) (manifest contracts.Manifest, err error) {
	gcsClient := shell.NewGoogleCloudStorageClient(shell.NewHTTPClient(), shell.StaticCredentials(credentials), []int{http.StatusOK})
	client := core.NewRetryClient(gcsClient, maxRetry, core.DefaultRetryDelay, logger)

	dependency := contracts.Dependency{PackageName: packageName, RemoteAddress: remoteAddress}
	address := dependency.ComposeLatestManifestRemoteAddress()
	body, err := client.Download(ctx, address)
	if contracts.IsNotFound(err) {
		return contracts.Manifest{}, fmt.Errorf("no root manifest found for package %q at [%s] (has the package been uploaded?): %w",
			packageName, address.String(), err)
	}
	if err != nil {
		return contracts.Manifest{}, fmt.Errorf("could not download root manifest for %q: %w", packageName, err)
	}
	defer func() { _ = body.Close() }()

	if err = json.NewDecoder(body).Decode(&manifest); err != nil {
		return contracts.Manifest{}, fmt.Errorf("could not decode manifest: %w", err)
	}
	return manifest, nil
}
