package satisfy

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/smarty/satisfy/v2/internal/model"
)

// Latest returns the most recently uploaded version of the package.
func (this *Client) Latest(ctx context.Context, ref PackageRef) (string, error) {
	manifest, err := this.rootManifest(ctx, ref)
	if err != nil {
		return "", err
	}
	return manifest.Version, nil
}

// Tags returns the package's tags, sorted by name.
func (this *Client) Tags(ctx context.Context, ref PackageRef) ([]Tag, error) {
	manifest, err := this.rootManifest(ctx, ref)
	if err != nil {
		return nil, err
	}
	tags := publicTags(manifest.Tags)
	slices.SortFunc(tags, func(a, b Tag) int { return strings.Compare(a.Name, b.Name) })
	return tags, nil
}

// Exists reports whether a manifest for the package version is on remote
// storage, using a HEAD request. version must be a literal version (tags are
// not resolved); "latest" reports whether any version has been uploaded.
func (this *Client) Exists(ctx context.Context, ref PackageRef, version string) (bool, error) {
	internal, err := validatePackageRef(ref)
	if err != nil {
		return false, err
	}
	if version == "" {
		return false, fmt.Errorf("%w: package version is required", ErrInvalidConfig)
	}
	_, err = this.storage.Size(ctx, internal.ComposeManifestAddress(version))
	if model.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (this *Client) rootManifest(ctx context.Context, public PackageRef) (model.Manifest, error) {
	ref, err := validatePackageRef(public)
	if err != nil {
		return model.Manifest{}, err
	}
	address := ref.ComposeRootManifestAddress()
	manifest, err := this.downloadManifest(ctx, address)
	if model.IsNotFound(err) {
		return model.Manifest{}, fmt.Errorf("no root manifest found for package %q at [%s] (has the package been uploaded?): %w",
			ref.Name, address.String(), err)
	}
	if err != nil {
		return model.Manifest{}, fmt.Errorf("could not read root manifest for package %q: %w", ref.Name, err)
	}
	return manifest, nil
}

// validatePackageRef reports an incomplete reference with an error matching
// ErrInvalidConfig and returns it, with its name normalized, for internal use.
func validatePackageRef(public PackageRef) (model.PackageRef, error) {
	ref := public.internal()
	switch {
	case ref.Name == "":
		return ref, fmt.Errorf("%w: package name is required", ErrInvalidConfig)
	case ref.RemoteAddress.Host == "":
		return ref, fmt.Errorf("%w: remote address is required", ErrInvalidConfig)
	}
	return ref, nil
}

// ModifyTags adds, moves and deletes tags of a package. Every version being
// tagged must already exist; otherwise nothing is changed.
func (this *Client) ModifyTags(ctx context.Context, public TagModification) error {
	if err := ValidateTagModification(public); err != nil {
		return err
	}
	modification := public.internal()
	address := modification.ComposeRootManifestAddress()
	rootManifest, err := this.downloadManifest(ctx, address)
	if model.IsNotFound(err) {
		return fmt.Errorf("no root manifest found for package %q at [%s] (has the package been uploaded?): %w",
			modification.PackageName, address.String(), err)
	}
	if err != nil {
		return fmt.Errorf("could not read root manifest: %w", err)
	}

	if err = this.verifyVersionsExist(ctx, modification); err != nil {
		return err
	}

	this.logModifications(rootManifest, modification)
	rootManifest.Tags = model.ApplyTagModifications(rootManifest.Tags, modification.Add, modification.Delete)
	if err = this.uploadManifest(ctx, rootManifest, address); err != nil {
		return err
	}
	this.logger.Info("tags updated", "package", modification.PackageName)
	return nil
}

// ValidateTagModification reports incomplete or inconsistent tag changes with
// an error matching ErrInvalidConfig. ModifyTags calls it before any I/O.
func ValidateTagModification(modification TagModification) error {
	switch {
	case modification.PackageName == "":
		return fmt.Errorf("%w: package name is required", ErrInvalidConfig)
	case modification.RemoteAddress.Host == "":
		return fmt.Errorf("%w: remote address is required", ErrInvalidConfig)
	case len(modification.Add) == 0 && len(modification.Delete) == 0:
		return fmt.Errorf("%w: at least one tag addition or deletion is required", ErrInvalidConfig)
	}
	added := make(map[string]struct{})
	for _, tag := range modification.Add {
		if tag.Name == "" || tag.Name == "latest" || tag.Version == "" {
			return fmt.Errorf("%w: invalid tag %q -> %q", ErrInvalidConfig, tag.Name, tag.Version)
		}
		if _, found := added[tag.Name]; found {
			return fmt.Errorf("%w: tag %q is added more than once", ErrInvalidConfig, tag.Name)
		}
		added[tag.Name] = struct{}{}
	}
	deleted := make(map[string]struct{})
	for _, tag := range modification.Delete {
		if tag.Name == "" {
			return fmt.Errorf("%w: invalid tag name to delete: %q", ErrInvalidConfig, tag.Name)
		}
		if _, found := added[tag.Name]; found {
			return fmt.Errorf("%w: tag %q is both added and deleted", ErrInvalidConfig, tag.Name)
		}
		if _, found := deleted[tag.Name]; found {
			return fmt.Errorf("%w: tag %q is deleted more than once", ErrInvalidConfig, tag.Name)
		}
		deleted[tag.Name] = struct{}{}
	}
	return nil
}

// verifyVersionsExist guards against dangling tags by confirming each version
// to be tagged has a manifest on remote storage.
func (this *Client) verifyVersionsExist(ctx context.Context, modification model.TagModificationConfig) error {
	verified := make(map[string]struct{})
	for _, tag := range modification.Add {
		if _, done := verified[tag.Version]; done {
			continue
		}
		address := modification.ComposeVersionedManifestAddress(tag.Version)
		_, err := this.storage.Size(ctx, address)
		if model.IsNotFound(err) {
			return fmt.Errorf("cannot tag version %q as %q: no manifest found at [%s]: %w",
				tag.Version, tag.Name, address.String(), err)
		}
		if err != nil {
			return fmt.Errorf("could not verify version %q exists: %w", tag.Version, err)
		}
		verified[tag.Version] = struct{}{}
	}
	return nil
}

func (this *Client) logModifications(rootManifest model.Manifest, modification model.TagModificationConfig) {
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
