package transfer

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/smarty/satisfy/v2/contracts"
	"github.com/smarty/satisfy/v2/core"
	"github.com/smarty/satisfy/v2/shell"
)

type TagsApp struct {
	config contracts.TagsConfig
	client contracts.RemoteStorage
	logger *slog.Logger
}

func NewTagsApp(config contracts.TagsConfig) *TagsApp {
	return &TagsApp{config: config, logger: contracts.LoggerOrDiscard(config.Logger)}
}

func (this *TagsApp) Run(ctx context.Context) error {
	this.buildRemoteStorageClient()
	modification := this.config.Modification

	rootManifest, err := this.downloadRootManifest(ctx)
	if err != nil {
		return err
	}

	err = this.verifyVersionsExist(ctx, modification.Add)
	if err != nil {
		return err
	}

	this.logModifications(rootManifest, modification)
	rootManifest.Tags = core.ApplyTagModifications(rootManifest.Tags, modification.Add, modification.Delete)

	err = this.uploadRootManifest(ctx, rootManifest)
	if err != nil {
		return err
	}

	this.logger.Info("tags updated", "package", modification.PackageName)
	return nil
}

func (this *TagsApp) buildRemoteStorageClient() {
	gcsClient := shell.NewGoogleCloudStorageClient(shell.NewHTTPClient(), shell.StaticCredentials(this.config.GoogleCredentials), []int{http.StatusOK})
	this.client = core.NewRetryClient(gcsClient, this.config.MaxRetry, core.DefaultRetryDelay, this.logger)
}

func (this *TagsApp) downloadRootManifest(ctx context.Context) (manifest contracts.Manifest, err error) {
	address := this.config.Modification.ComposeRootManifestAddress()
	body, err := this.client.Download(ctx, address)
	if contracts.IsNotFound(err) {
		return contracts.Manifest{}, fmt.Errorf(
			"no root manifest found for package %q at [%s] (has the package been uploaded?)",
			this.config.Modification.PackageName, address.String())
	}
	if err != nil {
		return contracts.Manifest{}, fmt.Errorf("could not download root manifest: %w", err)
	}
	defer func() { _ = body.Close() }()

	if err = json.NewDecoder(body).Decode(&manifest); err != nil {
		return contracts.Manifest{}, fmt.Errorf("could not decode root manifest: %w", err)
	}
	return manifest, nil
}

// verifyVersionsExist guards against dangling tags by confirming each version
// to be tagged has a manifest on remote storage. No modifications are written
// unless every version checks out.
func (this *TagsApp) verifyVersionsExist(ctx context.Context, add []contracts.Tag) error {
	verified := make(map[string]struct{})
	for _, tag := range add {
		if _, done := verified[tag.Version]; done {
			continue
		}
		address := this.config.Modification.ComposeVersionedManifestAddress(tag.Version)
		_, err := this.client.Size(ctx, address)
		if contracts.IsNotFound(err) {
			return fmt.Errorf("cannot tag version %q as %q: no manifest found at [%s]",
				tag.Version, tag.Name, address.String())
		}
		if err != nil {
			return fmt.Errorf("could not verify version %q exists: %w", tag.Version, err)
		}
		verified[tag.Version] = struct{}{}
	}
	return nil
}

func (this *TagsApp) logModifications(rootManifest contracts.Manifest, modification contracts.TagModificationConfig) {
	for _, tag := range modification.Add {
		if existing, found := rootManifest.TagVersion(tag.Name); found {
			this.logger.Info("updating tag", "tag", tag.Name, "from", existing, "to", tag.Version)
		} else {
			this.logger.Info("adding tag", "tag", tag.Name, "version", tag.Version)
		}
	}
	for _, tag := range modification.Delete {
		if _, found := rootManifest.TagVersion(tag.Name); found {
			this.logger.Info("deleting tag", "tag", tag.Name)
		} else {
			this.logger.Info("tag does not exist; nothing to delete", "tag", tag.Name)
		}
	}
}

func (this *TagsApp) uploadRootManifest(ctx context.Context, manifest contracts.Manifest) error {
	buffer := new(bytes.Buffer)
	hasher := md5.New()
	encoder := json.NewEncoder(io.MultiWriter(buffer, hasher))
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		return err
	}
	return this.client.Upload(ctx, contracts.UploadRequest{
		RemoteAddress: this.config.Modification.ComposeRootManifestAddress(),
		Body:          bytes.NewReader(buffer.Bytes()),
		Size:          int64(buffer.Len()),
		ContentType:   "application/json",
		Checksum:      hasher.Sum(nil),
	})
}
