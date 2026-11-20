// Package satisfy uploads, downloads and verifies versioned package archives
// stored in Google Cloud Storage.
//
// A package is a compressed archive (zstd, gzip or zip) plus a JSON manifest
// that lists every file and its checksum. Installing a package downloads the
// archive into a local directory and records the manifest there, so later
// installs can verify the files and skip the download when nothing changed.
//
// Create a Client and call its methods:
//
//	client := satisfy.New(satisfy.WithLogger(slog.Default()))
//
//	err := client.Install(ctx, satisfy.Dependency{
//		PackageName:    "my/package",
//		PackageVersion: "1.2.3",
//		RemoteAddress:  satisfy.URL{Scheme: "gcs", Host: "my-bucket"},
//		LocalDirectory: "deps/my-package",
//	})
//
// The library never exits the process or writes to stdout or stderr: failures
// are returned as errors, logging is off unless WithLogger is given, and
// progress is reported only to a WithProgress callback.
package satisfy
