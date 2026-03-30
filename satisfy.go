package satisfy

import (
	"iter"

	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/internal/transfer"
)

// Check verifies whether the package described by config already exists on
// remote storage by attempting to download its manifest.
//
// If config.Overwrite is true, the remote check is skipped and an
// EventInfo is emitted. Otherwise, the sequence yields ErrPackageExists
// if the package is found, a wrapped error on unexpected failure, and
// completes normally if the package does not yet exist.
func Check(config contracts.CheckConfiguration) iter.Seq2[contracts.Event, error] {
	return transfer.NewCheckApp(config).Run
}

// Download installs all package dependencies listed in config.Dependencies.
//
// Each dependency is resolved concurrently. EventProgress events are emitted
// as archive items are extracted. EventFailure events are emitted for
// individual package failures. A terminal error is yielded if one or more
// installations fail.
func Download(config contracts.DownloadConfiguration) iter.Seq2[contracts.Event, error] {
	return transfer.NewDownloadApp(config).Run
}

// Upload compresses the source directory into an archive and uploads it along
// with its manifest to remote storage.
//
// Unless config.Overwrite is true, a pre-upload check is performed first;
// ErrPackageExists is yielded if the package already exists. EventInfo and
// EventProgress events are emitted throughout the archive build and upload.
// A terminal error is yielded on any failure.
func Upload(config contracts.UploadConfiguration) iter.Seq2[contracts.Event, error] {
	return transfer.NewUploadApp(config).Run
}
