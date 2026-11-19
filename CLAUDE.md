# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

`satisfy` is a Go package manager that uploads, downloads, and verifies versioned package archives to/from Google Cloud Storage (GCS). It manages manifests (`manifest.json`) and compressed archives (gzip, zstd, zip) stored remotely, with local integrity checking to skip unnecessary re-downloads. It is both a CLI and an importable library.

## Repository Layout: v0 and v2

- The repository root is the **deprecated v0 module** (`github.com/smarty/satisfy`).
- **All development happens in `v2/`**, a separate module: `github.com/smarty/satisfy/v2` (note: `smarty`, not `smartystreets`).

## Build & Test Commands

The Makefile runs Go in `v2/` (`go -C v2 ...`).

```bash
make test        # go mod tidy + go fmt, then tests
make coverage    # race detector + atomic coverage
make compile     # build ./workspace/satisfy (uses OS/CPU env vars)
make build       # coverage + compile
make image       # Docker image

# directly
go -C v2 test -short ./...
go -C v2 test -short -run TestName ./internal/packaging/
```

## Architecture (v2)

- **`satisfy` (module root, `v2/*.go`)** — the public API. `satisfy.New(options...)` returns a `*Client` with `Install`, `Upload`, `Exists`, `Latest`, `Tags`, `ModifyTags`. Options: `WithCredentials`, `WithCredentialsReader`, `WithHTTPClient`, `WithMaxRetry`, `WithRetryDelay`, `WithConcurrency`, `WithContentVerification`, `WithLogger`, `WithProgress`. Errors: `ErrPackageExists`, `ErrNotFound`, `ErrInvalidConfig`, `*StatusError`. `ValidatePackageConfig` (incl. exactly one of source path/directory/file) and `ValidateTagModification` check input before any I/O (the CLI uses them too). `Latest`, `Tags` and `Exists` identify a package with a `PackageRef` (remote address + name). Remote addresses are `URL` values everywhere (never pointers); `ParseURL` builds one from a string. Domain types are aliases of `internal/model` types.
- **`internal/model`** — domain types (`Manifest`, `PackageRef`, `Dependency`, `PackageConfig`, `TagModificationConfig`, `URL`, `Progress`), errors (`ErrNotFound`, `ErrPackageExists`, `ErrRetryable`, `StatusCodeError`), small interfaces (`RemoteStorage`, file system interfaces, `ArchiveWriter`, `PackageInstaller`, `IntegrityCheck`), and tag merge helpers.
- **`internal/storage`** — GCS client, `CredentialsSource` (`StaticCredentials`, `RefreshingCredentials`), HTTP client, and `RetryClient` (context-aware waits, rewinds upload bodies before each attempt).
- **`internal/packaging`** — `DirectoryPackageBuilder`, `PackageInstaller` (extract + checksum + rollback), `DependencyResolver` (versions, `latest`, tags), integrity checks.
- **`internal/archive`** — tar/zip writers, zip reader, parallel zstd writer, progress counter.
- **`internal/fsys`** — `DiskFileSystem`.
- **`cmd/satisfy`** — the CLI. `main` calls `run(ctx, args, stdin, stdout, stderr) int`. Owns flag parsing, JSON config loading (upload/tags loaders), credential discovery (incl. Vault), the slog text logger on stderr, the progress renderer, and exit codes (0 ok / `-h`, 1 failure, 2 package exists).

## Library Rules

Library code (everything outside `cmd/` and tests) must never call `os.Exit`, `log.Fatal*`, `log.Panic*` or `panic`, import `log` or `flag`, use `os.Stdout`/`os.Stderr`/`os.Stdin`/`os.Args`, `fmt.Print*` or `slog.Default`. Return errors; take a `context.Context` first; log only through an injected `*slog.Logger` (nil means silent, via `model.LoggerOrDiscard`); report progress through `model.ProgressFunc`.

## Testing Conventions

All tests use [`gunit`](https://pkg.go.dev/github.com/smarty/gunit) fixtures with [`assertions`](https://pkg.go.dev/github.com/smarty/assertions) — including CLI tests. Pattern:

```go
func TestSomethingFixture(t *testing.T) {
    gunit.Run(new(SomethingFixture), t)
}

type SomethingFixture struct {
    *gunit.Fixture
}

func (this *SomethingFixture) Setup() { /* ... */ }
func (this *SomethingFixture) TestSomeBehavior() {
    this.So(actual, should.Equal, expected)
}
```

`client_test.go` exercises the `Client` end to end against an in-memory `fakeStorage` (injected with the unexported `withStorage` option).

## Style Notes

- Receiver variable is always `this`.
- Interfaces are small and composed via embedding at usage sites (e.g. `PackageInstallerFileSystem`).
- Constructors do no I/O.
