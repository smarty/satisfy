package satisfy

import "github.com/smarty/satisfy/v2/internal/model"

type (
	// URL is a remote storage address such as gcs://bucket/path/prefix. It
	// marshals to and from a JSON string.
	URL = model.URL

	// Dependency describes a package version to install into a local directory.
	// PackageVersion may be a version, a tag, or "latest".
	Dependency = model.Dependency

	// DependencyListing is the JSON document listing the dependencies to install.
	DependencyListing = model.DependencyListing

	// PackageConfig describes a package version to build and upload.
	PackageConfig = model.PackageConfig

	// Manifest describes an uploaded package version and its archive.
	Manifest = model.Manifest

	// Archive describes the compressed archive of a package version.
	Archive = model.Archive

	// ArchiveItem describes one file in an archive.
	ArchiveItem = model.ArchiveItem

	// Tag names a package version, e.g. "stable" -> "1.2.3".
	Tag = model.Tag

	// TagModification adds, moves and deletes tags of a package.
	TagModification = model.TagModificationConfig

	// Progress reports how many bytes of one item have been processed.
	Progress = model.Progress

	// ProgressPhase names the step a Progress report describes.
	ProgressPhase = model.ProgressPhase

	// StatusError is returned when remote storage responds with an unexpected
	// HTTP status code.
	StatusError = model.StatusCodeError
)

const (
	ProgressArchiving   = model.ProgressArchiving
	ProgressDownloading = model.ProgressDownloading
	ProgressExtracting  = model.ProgressExtracting
)
