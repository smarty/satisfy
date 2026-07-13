package transfer

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/core"
	"github.com/smarty/satisfy/shell"
)

type TagsApp struct {
	config contracts.TagsConfig
	client contracts.RemoteStorage
}

func NewTagsApp(config contracts.TagsConfig) *TagsApp {
	return &TagsApp{config: config}
}

func (this *TagsApp) Run() {
	if err := this.TryRun(); err != nil {
		log.Fatal(err)
	}
}

func (this *TagsApp) TryRun() error {
	this.buildRemoteStorageClient()
	modification := this.config.Modification

	rootManifest, err := this.downloadRootManifest()
	if err != nil {
		return err
	}

	err = this.verifyVersionsExist(modification.Add)
	if err != nil {
		return err
	}

	this.logModifications(rootManifest, modification)
	rootManifest.Tags = core.ApplyTagModifications(rootManifest.Tags, modification.Add, modification.Delete)

	err = this.uploadRootManifest(rootManifest)
	if err != nil {
		return err
	}

	log.Printf("Tags updated for package %q", modification.PackageName)
	return nil
}

func (this *TagsApp) buildRemoteStorageClient() {
	gcsClient := shell.NewGoogleCloudStorageClient(shell.NewHTTPClient(), this.config.GoogleCredentials, []int{http.StatusOK})
	this.client = core.NewRetryClient(gcsClient, this.config.MaxRetry, time.Sleep)
}

func (this *TagsApp) downloadRootManifest() (manifest contracts.Manifest, err error) {
	address := this.config.Modification.ComposeRootManifestAddress()
	body, err := this.client.Download(address)
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
func (this *TagsApp) verifyVersionsExist(add []contracts.Tag) error {
	verified := make(map[string]struct{})
	for _, tag := range add {
		if _, done := verified[tag.Version]; done {
			continue
		}
		address := this.config.Modification.ComposeVersionedManifestAddress(tag.Version)
		_, err := this.client.Size(address)
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
			log.Printf("Updating tag %q: %q -> %q", tag.Name, existing, tag.Version)
		} else {
			log.Printf("Adding tag %q -> %q", tag.Name, tag.Version)
		}
	}
	for _, tag := range modification.Delete {
		if _, found := rootManifest.TagVersion(tag.Name); found {
			log.Printf("Deleting tag %q", tag.Name)
		} else {
			log.Printf("Tag %q does not exist; nothing to delete", tag.Name)
		}
	}
}

func (this *TagsApp) uploadRootManifest(manifest contracts.Manifest) error {
	buffer := new(bytes.Buffer)
	hasher := md5.New()
	encoder := json.NewEncoder(io.MultiWriter(buffer, hasher))
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		return err
	}
	return this.client.Upload(contracts.UploadRequest{
		RemoteAddress: this.config.Modification.ComposeRootManifestAddress(),
		Body:          bytes.NewReader(buffer.Bytes()),
		Size:          int64(buffer.Len()),
		ContentType:   "application/json",
		Checksum:      hasher.Sum(nil),
	})
}
