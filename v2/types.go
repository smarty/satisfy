package satisfy

import "github.com/smarty/satisfy/v2/contracts"

type (
	// URL is a remote storage address such as gcs://bucket/path/prefix. It
	// marshals to and from a JSON string.
	URL = contracts.URL

	// Dependency describes a package version to install into a local directory.
	// PackageVersion may be a version, a tag, or "latest".
	Dependency = contracts.Dependency

	// DependencyListing is the JSON document listing the dependencies to install.
	DependencyListing = contracts.DependencyListing

	// PackageConfig describes a package version to build and upload.
	PackageConfig = contracts.PackageConfig

	// Manifest describes an uploaded package version and its archive.
	Manifest = contracts.Manifest

	// Archive describes the compressed archive of a package version.
	Archive = contracts.Archive

	// ArchiveItem describes one file in an archive.
	ArchiveItem = contracts.ArchiveItem

	// Tag names a package version, e.g. "stable" -> "1.2.3".
	Tag = contracts.Tag

	// TagModification adds, moves and deletes tags of a package.
	TagModification = contracts.TagModificationConfig

	// Progress reports how many bytes of one item have been processed.
	Progress = contracts.Progress

	// ProgressPhase names the step a Progress report describes.
	ProgressPhase = contracts.ProgressPhase

	// StatusError is returned when remote storage responds with an unexpected
	// HTTP status code.
	StatusError = contracts.StatusCodeError
)

const (
	ProgressArchiving   = contracts.ProgressArchiving
	ProgressDownloading = contracts.ProgressDownloading
	ProgressExtracting  = contracts.ProgressExtracting
)
