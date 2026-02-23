# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

`satisfy` is a Go-based package dependency manager that uploads, downloads, and verifies versioned package archives to/from Google Cloud Storage (GCS). It manages manifests (`manifest.json`) and compressed archives (gzip, zstd, zip) stored remotely, with local integrity checking to skip unnecessary re-downloads.

## Build & Test Commands

```bash
# Run tests (formats code first via go fmt, then runs tests)
make test

# Run tests directly
go test -timeout=1s -short -cover ./...

# Run a single test
go test -timeout=1s -short -run TestName ./core/

# Race detector + atomic coverage
make coverage

# Format and tidy
make fmt

# Compile binary
make compile          # uses OS/CPU env vars
make build            # coverage + compile

# Build Docker image
make image
```

## Architecture

The module path is `github.com/smarty/satisfy` (note: **not** `smartystreets`).

### Package Layers

- **`contracts/`** — Shared types and interfaces. All domain structs live here: `Manifest`, `Archive`, `Dependency`, `DependencyListing`, `UploadConfig`, `PackageConfig`. Also defines interfaces for filesystem operations (`FileCreator`, `FileReader`, `Deleter`, etc.), remote storage (`Uploader`, `Downloader`), and integrity checking (`IntegrityCheck`, `PackageInstaller`).

- **`core/`** — Business logic with no external I/O. Key types:
  - `PackageInstaller` — downloads manifests and extracts archives (tar over gzip/zstd, or zip) into local directories, with checksum verification and filesystem rollback on failure.
  - `DependencyResolver` — decides whether a dependency needs installation by checking local manifests and running integrity checks, then delegates to `PackageInstaller`.
  - `PackageBuilder` — builds archive + manifest content from a source directory.
  - `RetryClient` — wraps a `RemoteStorage` with configurable retry logic.
  - Integrity checkers: `FileListingIntegrityChecker`, `FileContentIntegrityCheck`, `CompoundIntegrityCheck`.

- **`shell/`** — Infrastructure adapters (no business logic): `DiskFileSystem`, `Environment`, `GoogleCloudStorageClient`, `HTTPClient`, `TarArchiveWriter`, `ZipArchiveWriter`, `ZipArchiveReader`.

- **`transfer/`** — Application-level orchestrators wiring core + shell together:
  - `UploadApp` — builds archive, computes manifest, uploads both to GCS.
  - `DownloadApp` — resolves dependencies concurrently (goroutine per dependency).
  - `CheckApp` — pre-upload check for existing remote manifests.
  - `ParseDownloadConfig` — CLI flag parsing and dependency listing loading.

- **`cmd/satisfy/`** — CLI entry point. Subcommands: `upload`, `check`, `version`; default (no subcommand) is `download`.

### Testing Conventions

Tests use the [`gunit`](https://pkg.go.dev/github.com/smarty/gunit) framework with fixture-based test structs and [`assertions`](https://pkg.go.dev/github.com/smarty/assertions) for `should`-style assertions. Pattern:

```go
func TestSomethingFixture(t *testing.T) {
    gunit.Run(new(SomethingFixture), t)
}

type SomethingFixture struct {
    *gunit.Fixture
    // fields
}

func (this *SomethingFixture) Setup() { /* ... */ }
func (this *SomethingFixture) TestSomeBehavior() {
    this.So(actual, should.Equal, expected)
}
```

### Style Notes

- Receiver variable is always `this`.
- Interfaces are defined granularly in `contracts/` (single-method where possible) and composed via embedding at usage sites.
- `core/` types define their own filesystem interface (e.g., `PackageInstallerFileSystem`, `DependencyResolverFileSystem`) by embedding only the `contracts` interfaces they need.
