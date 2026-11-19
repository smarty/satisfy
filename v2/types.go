package satisfy

import (
	"net/url"

	"github.com/smarty/satisfy/v2/internal/model"
)

// URL is a remote storage address such as gcs://bucket/path/prefix. It
// marshals to and from a JSON string. Build one directly, e.g.
// URL{Scheme: "gcs", Host: "bucket"}, or with ParseURL.
type URL url.URL

// String returns the address in URL form, e.g. "gcs://bucket/prefix".
func (this URL) String() string { return model.URL(this).String() }

// MarshalJSON writes the address as a JSON string. It has a value receiver so
// that a URL is written as a string even within a struct marshaled by value.
func (this URL) MarshalJSON() ([]byte, error) { return model.URL(this).MarshalJSON() }

// UnmarshalJSON reads the address from a JSON string. A JSON null (or the
// string "null") leaves the URL unchanged.
func (this *URL) UnmarshalJSON(raw []byte) error { return (*model.URL)(this).UnmarshalJSON(raw) }

// PackageRef identifies a package on remote storage.
type PackageRef struct {
	// RemoteAddress is the address under which packages live, e.g.
	// gcs://bucket/prefix.
	RemoteAddress URL `json:"remote_address"`

	// Name is the package's name, e.g. "my/package". Leading and trailing
	// slashes are ignored.
	Name string `json:"package_name"`
}

// Dependency describes a package version to install into a local directory.
type Dependency struct {
	// PackageName is the name of the package to install.
	PackageName string `json:"package_name"`

	// PackageVersion is a version, a tag, or "latest" (the most recently
	// uploaded version).
	PackageVersion string `json:"package_version"`

	// RemoteAddress is the address under which the package lives, e.g.
	// gcs://bucket/prefix.
	RemoteAddress URL `json:"remote_address"`

	// LocalDirectory is the directory the package is installed into. It is
	// used as given: it is not expanded by a shell, so "~" is not the home
	// directory.
	LocalDirectory string `json:"local_directory"`
}

// PackageConfig describes a package version to build and upload.
type PackageConfig struct {
	// CompressionAlgorithm is "zstd", "gzip" or "zip".
	CompressionAlgorithm string `json:"compression_algorithm"`

	// CompressionLevel is the algorithm's compression level; 0 selects the
	// algorithm's default level.
	CompressionLevel int `json:"compression_level"`

	// SourceDirectory, SourceFile and SourcePath name the file or directory
	// to archive. They are interchangeable; exactly one must be given. A
	// directory is archived with its contents; a single file is archived at
	// the archive's root.
	SourceDirectory string `json:"source_directory"`
	SourceFile      string `json:"source_file"`
	SourcePath      string `json:"source_path"`

	// PackageName is the name of the package, e.g. "my/package".
	PackageName string `json:"package_name"`

	// PackageVersion is the version being uploaded.
	PackageVersion string `json:"package_version"`

	// RemoteAddress is the address under which packages live, e.g.
	// gcs://bucket/prefix.
	RemoteAddress URL `json:"remote_address"`

	// Tags are pointed at the uploaded version, e.g. "stable". Optional.
	Tags []string `json:"tags,omitempty"`
}

// TagModification adds, moves and deletes tags of a package.
type TagModification struct {
	// PackageName is the name of the package whose tags change.
	PackageName string `json:"package_name"`

	// RemoteAddress is the address under which the package lives, e.g.
	// gcs://bucket/prefix.
	RemoteAddress URL `json:"remote_address"`

	// Add creates each tag, or moves it to a new version when it exists.
	Add []Tag `json:"add"`

	// Delete removes each tag. Only the Name of each entry is used.
	Delete []Tag `json:"delete"`
}

// Tag names a package version, e.g. "stable" -> "1.2.3".
type Tag struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Manifest describes an uploaded package version and its archive.
type Manifest struct {
	// Name is the package's name.
	Name string `json:"name"`

	// Version is the package version the manifest describes.
	Version string `json:"version"`

	// Archive describes the compressed archive holding the package's files.
	Archive Archive `json:"archive"`

	// Tags lists the package's tags. Only the root manifest, which describes
	// the latest version, has tags.
	Tags []Tag `json:"tags,omitempty"`
}

// Archive describes the compressed archive of a package version.
type Archive struct {
	// Filename is the archive's name on remote storage.
	Filename string `json:"filename"`

	// Size is the size of the compressed archive in bytes.
	Size uint64 `json:"size"`

	// MD5Checksum is the MD5 checksum of the compressed archive.
	MD5Checksum []byte `json:"md5"`

	// Contents lists every file in the archive.
	Contents []ArchiveItem `json:"contents"`

	// CompressionAlgorithm is "zstd", "gzip" or "zip".
	CompressionAlgorithm string `json:"compression"`
}

// ArchiveItem describes one file in an archive.
type ArchiveItem struct {
	// Path is the file's path relative to the installation directory.
	Path string `json:"path"`

	// Size is the file's size in bytes. For a symbolic link it is the length
	// of the link's target.
	Size int64 `json:"size"`

	// MD5Checksum is the MD5 checksum of the file's contents, or of the
	// link's target for a symbolic link.
	MD5Checksum []byte `json:"md5"`
}

// ProgressPhase names the step a Progress report describes.
type ProgressPhase string

const (
	ProgressArchiving   ProgressPhase = "archiving"
	ProgressDownloading ProgressPhase = "downloading"
	ProgressExtracting  ProgressPhase = "extracting"
)

// Progress reports how many bytes of one item have been processed. Reports
// arrive periodically while work is under way and once more, with Done set,
// when the item is finished.
type Progress struct {
	// Phase is the step under way.
	Phase ProgressPhase

	// Name identifies the item, such as a file being archived or extracted.
	Name string

	// Written is the number of bytes processed so far.
	Written int64

	// Total is the number of bytes expected, or 0 when unknown.
	Total int64

	// Done is set on the final report for the item.
	Done bool
}

// StatusError is returned when remote storage responds with an unexpected
// HTTP status code.
type StatusError = model.StatusCodeError
