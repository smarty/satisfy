package satisfy

import (
	"github.com/smarty/satisfy/configuration"
	"github.com/smarty/satisfy/internal/transfer"
)

// Check verifies whether the package described by config already exists on
// remote storage by attempting to download its manifest.
//
// If config.Overwrite is true, the remote check is skipped and Check returns
// immediately. Otherwise, it exits with code 2 if the package is found,
// code 1 on an unexpected error, and returns normally if the package does
// not yet exist.
//
// Parameters:
//   - config: the package identity and GCS credentials used to locate the
//     remote manifest.
func Check(config configuration.CheckConfiguration) {
	transfer.NewCheckApp(config).Run()
}

// Download installs all package dependencies listed in config.Dependencies.
//
// Each dependency is resolved concurrently. If the package is already
// installed at the correct version and passes its integrity check, it is left
// in place. Otherwise the existing installation is removed and the package is
// re-downloaded. Exits with code 1 if one or more installations fail.
//
// Parameters:
//   - config: the dependency listing, GCS credentials, retry limit, and
//     verification settings that control installation behavior.
func Download(config configuration.DownloadConfiguration) {
	transfer.NewDownloadApp(config).Run()
}

// Upload compresses the source directory into an archive and uploads it along
// with its manifest to remote storage.
//
// Unless config.Overwrite is true, a pre-upload check is performed first: if
// the package already exists on remote storage the process exits with code 2.
// After archiving, if compression took longer than 30 minutes the GCS bearer
// token is refreshed before uploading. The archive and two copies of the
// manifest (one versioned, one at the "latest" path) are then uploaded.
// Exits with code 1 on any failure.
//
// Parameters:
//   - config: the package metadata, source path, compression settings, GCS
//     credentials, and upload options that control the archiving and upload.
func Upload(config configuration.UploadConfiguration) {
	transfer.NewUploadApp(config).Run()
}
