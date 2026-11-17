package transfer

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/smarty/gcs"

	"github.com/smarty/satisfy/v2/contracts"
)

type TagsListConfig struct {
	PackageName       string
	RemoteAddress     contracts.URL
	GoogleCredentials gcs.Credentials
	MaxRetry          int
	Logger            *slog.Logger
}

type TagsListApp struct {
	config TagsListConfig
}

func NewTagsListApp(config TagsListConfig) *TagsListApp {
	return &TagsListApp{config: config}
}

// Run returns the package's tags, sorted by name.
func (this *TagsListApp) Run(ctx context.Context) ([]contracts.Tag, error) {
	manifest, err := downloadRootManifest(ctx, this.config.GoogleCredentials, this.config.MaxRetry, this.config.Logger,
		this.config.RemoteAddress, this.config.PackageName)
	if err != nil {
		return nil, err
	}
	tags := slices.Clone(manifest.Tags)
	slices.SortFunc(tags, func(a, b contracts.Tag) int { return strings.Compare(a.Name, b.Name) })
	return tags, nil
}
