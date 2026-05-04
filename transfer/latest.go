package transfer

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/smarty/gcs"

	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/core"
	"github.com/smarty/satisfy/shell"
)

type LatestConfig struct {
	PackageName       string
	RemoteAddress     contracts.URL
	GoogleCredentials gcs.Credentials
	MaxRetry          int
}

func ParseLatestConfig(args []string) (config LatestConfig, err error) {
	flags := flag.NewFlagSet("satisfy", flag.ContinueOnError)

	var bucket, prefix, packageName string
	flags.StringVar(&bucket,
		"bucket",
		"",
		"GCS bucket name (required), e.g. liveaddress-downloads-dev.",
	)
	flags.StringVar(&prefix,
		"path",
		"",
		"Optional path prefix within the bucket where packages live, e.g. /releases.",
	)
	flags.StringVar(&packageName,
		"package",
		"",
		"Package name (required), e.g. master-address-list/2026/04/premium/az.",
	)
	flags.IntVar(&config.MaxRetry,
		"max-retry",
		5,
		"How many times to retry the manifest download.",
	)

	flags.Usage = func() {
		output := flags.Output()
		_, _ = fmt.Fprintf(output, "Usage: %s latest -bucket <name> [-path <prefix>] -package <name>\n\n", os.Args[0])
		flags.PrintDefaults()
	}

	if err = flags.Parse(args); err != nil {
		return LatestConfig{}, err
	}

	if bucket == "" {
		return LatestConfig{}, errors.New("-bucket is required")
	}
	if strings.Contains(bucket, "/") {
		return LatestConfig{}, errors.New("-bucket should be a bare bucket name (no scheme, no slashes)")
	}

	config.PackageName = strings.Trim(packageName, "/")
	if config.PackageName == "" {
		return LatestConfig{}, errors.New("-package is required")
	}

	config.RemoteAddress = contracts.URL{Scheme: "gcs", Host: bucket}
	if prefix = strings.Trim(prefix, "/"); prefix != "" {
		config.RemoteAddress.Path = "/" + prefix
	}

	reader := gcs.NewCredentialsReader()
	config.GoogleCredentials, err = reader.Read(context.Background(), "")
	if err != nil {
		return LatestConfig{}, fmt.Errorf("could not load Google credentials: %w", err)
	}
	return config, nil
}

type LatestApp struct {
	config LatestConfig
}

func NewLatestApp(config LatestConfig) *LatestApp {
	return &LatestApp{config: config}
}

func (this *LatestApp) Run() {
	if err := this.TryRun(); err != nil {
		log.Fatal(err)
	}
}

func (this *LatestApp) TryRun() error {
	gcsClient := shell.NewGoogleCloudStorageClient(shell.NewHTTPClient(), this.config.GoogleCredentials, []int{http.StatusOK})
	client := core.NewRetryClient(gcsClient, this.config.MaxRetry, time.Sleep)

	dependency := contracts.Dependency{
		PackageName:   this.config.PackageName,
		RemoteAddress: this.config.RemoteAddress,
	}

	body, err := client.Download(dependency.ComposeLatestManifestRemoteAddress())
	if err != nil {
		return fmt.Errorf("could not download latest manifest for %q: %w", this.config.PackageName, err)
	}
	defer func() { _ = body.Close() }()

	var manifest contracts.Manifest
	if err = json.NewDecoder(body).Decode(&manifest); err != nil {
		return fmt.Errorf("could not decode manifest: %w", err)
	}

	fmt.Println(manifest.Version)
	return nil
}
