# Plan: Event-based API with iter.Seq2

## Context

The library currently uses a `Logger` interface (10+ methods, including `Fatal`/`os.Exit`) threaded through `internal/transfer` and exposed in `contracts`. This couples CLI concerns (log prefixes, exit codes, writers) into the library core. The goal is to replace all internal logging and fatal exits with a clean event stream so library callers receive `iter.Seq2[contracts.Event, error]` — events for progress/warnings/failures, a terminal `error` when the operation must stop. The CLI owns all exit-code decisions in one place.

## Go version bump

`go.mod`: `go 1.22` → `go 1.23` (required for `iter.Seq2` and range-over-func).

---

## Step 1 — Add `contracts/event.go`

New file. Ordering: constants (`eventUnknown` first to give zero value a safe sentinel), then type, then Event struct.

```go
type EventType int

const (
    eventUnknown EventType = iota
    EventProgress
    EventInfo
    EventWarning
    EventFailure
)

type Event struct {
    Message string
    Type    EventType
}
```

## Step 2 — Add `ErrPackageExists` to `contracts/errors.go`

Add to the existing var block (alphabetical order puts it between `ErrNilRemoteAddressPrefix` and `ErrNoDependenciesMatch`):

```go
ErrPackageExists = errors.New("package already exists")
```

## Step 3 — Delete files

- `contracts/logger.go`
- `contracts/log_level.go`
- `internal/logging/logger.go`
- `internal/logging/level.go`

---

## Step 4 — `internal/core/builder.go` and `internal/core/installer.go`

Replace `log.Printf` progress calls with an `emit func(contracts.Event)` callback added to each constructor. The callback is nil-safe (no-op if nil).

**`DirectoryPackageBuilder`**: add `emit func(contracts.Event)` field; `NewDirectoryPackageBuilder` gains the parameter. Replace `log.Printf("Adding \"%s\" to archive.", ...)` with `this.emit(contracts.Event{Type: contracts.EventProgress, Message: ...})`.

**`PackageInstaller`**: same pattern. Replace `log.Printf("Extracting archive item [%d/%d]...")` with emit call.

Both constructors already have a `newProgress` parameter as precedent for optional callbacks.

---

## Step 5 — `internal/transfer/check.go`

Remove `logger contracts.Logger` field and constructor parameter.

`Run` becomes the `iter.Seq2` function directly:

```go
func (this *CheckApp) Run(yield func(contracts.Event, error) bool) {
    if this.config.Overwrite {
        yield(contracts.Event{Type: contracts.EventInfo, Message: "Overwrite mode enabled, skipping remote manifest check."}, nil)
        return
    }
    body, err := this.buildRemoteStorageClient().Download(...)
    if err == nil {
        _ = body.Close()
        return
    }
    if code, ok := contracts.StatusCode(err); ok && code == http.StatusOK {
        yield(contracts.Event{}, contracts.ErrPackageExists)
        return
    }
    yield(contracts.Event{}, fmt.Errorf("sanity check failed: %w", err))
}
```

---

## Step 6 — `internal/transfer/upload.go`

Remove `logger`. Add `emit func(contracts.Event)` (passed into `NewDirectoryPackageBuilder`).

Fold `runPreUploadCheck` into `Run` — it can no longer call `os.Exit`. Return `contracts.ErrPackageExists` when the package already exists.

`Run` signature becomes `func(yield func(contracts.Event, error) bool)`. Every `log.Fatal(err)` becomes `yield(contracts.Event{}, err); return`. Every `log.Println(...)` becomes `yield(contracts.Event{Type: contracts.EventInfo/EventProgress, Message: ...}, nil)`.

The emit callback is threaded into `core.NewDirectoryPackageBuilder` so builder progress events flow back through the same yield.

---

## Step 7 — `internal/transfer/download.go`

Remove `logger contracts.Logger` field.

**Orchestrator struct** (private to this file): wraps the results channel with an `atomic.Bool` closed flag. Prevents goroutines from sending to the channel after the consumer stops iterating.

```go
type downloadOrchestrator struct {
    results chan error
    events  chan contracts.Event
    closed  atomic.Bool
}
func (o *downloadOrchestrator) emitEvent(e contracts.Event) { /* no-op if closed */ }
func (o *downloadOrchestrator) emitError(err error)         { /* no-op if closed */ }
func (o *downloadOrchestrator) cancel()                     { o.closed.Store(true) }
```

`Run` starts all install goroutines, starts one goroutine that calls `waiter.Wait()` then closes both channels, then loops selecting over both channels, yielding `EventFailure` events for individual install errors, and yielding a terminal `error` at the end if any installs failed. If `yield` returns `false`, call `orch.cancel()` and return.

The emit callback passed into `core.NewPackageInstaller` sends to `orch.events`.

`TryRun` is removed (it only existed to separate testable logic from the fatal call — no longer needed).

---

## Step 8 — `satisfy.go`

Remove `io.Writer` and `exitFunc func(int)` parameters entirely. Return `iter.Seq2[contracts.Event, error]`.

```go
func Check(config contracts.CheckConfiguration) iter.Seq2[contracts.Event, error] {
    return transfer.NewCheckApp(config).Run
}
func Download(config contracts.DownloadConfiguration) iter.Seq2[contracts.Event, error] {
    return transfer.NewDownloadApp(config).Run
}
func Upload(config contracts.UploadConfiguration) iter.Seq2[contracts.Event, error] {
    return transfer.NewUploadApp(config).Run
}
```

Update doc comments to describe the event stream rather than exit codes.

---

## Step 9 — `cmd/satisfy/handle_errors.go` (new file)

Contains per-command handlers and a shared `printEvent` helper. The CLI is the only place that knows exit codes.

```go
func handleCheck(seq iter.Seq2[contracts.Event, error])
func handleDownload(seq iter.Seq2[contracts.Event, error])
func handleUpload(seq iter.Seq2[contracts.Event, error])
func printEvent(event contracts.Event)
```

Each handler: ranges over the sequence, prints events via `printEvent`, and on a non-nil error checks for sentinel values (`ErrPackageExists` → exit 2, anything else → exit 1).

`printEvent` switches on `event.Type` and writes to stdout (EventProgress) or stderr (everything else) with appropriate prefixes (`[INFO]`, `[WARN]`, `[ERROR]`).

---

## Step 10 — `cmd/satisfy/main.go` and `cmd/satisfy/parse.go`

Remove `var logger = contracts.NewLogger(...)`.

Replace all `logger.*` calls:
- `logger.LogLineClean(format, ...)` → `fmt.Fprintf(os.Stderr, format+"\n", ...)`
- `logger.LogLine(level, ...)` → `fmt.Fprintf(os.Stderr, "[LEVEL] "+format+"\n", ...)`
- `logger.LogClean(...)` → `fmt.Fprintf(os.Stderr, format, ...)`
- `logger.WriterErr()` in `flags.SetOutput(...)` → `os.Stderr`
- `logger.Fatal(err)` in `main.go` → replaced by `handleCheck`/`handleDownload`/`handleUpload`

`mainCheck`, `mainDownload`, `mainUpload` now call `handleCheck(satisfy.Check(config))` etc.

---

## Files changed

| File | Action |
|------|--------|
| `go.mod` | bump to go 1.23 |
| `contracts/event.go` | create |
| `contracts/errors.go` | add ErrPackageExists |
| `contracts/logger.go` | delete |
| `contracts/log_level.go` | delete |
| `internal/logging/logger.go` | delete |
| `internal/logging/level.go` | delete |
| `satisfy.go` | return iter.Seq2, remove io.Writer/exitFunc params |
| `internal/transfer/check.go` | remove logger, Run becomes iter.Seq2 method |
| `internal/transfer/upload.go` | remove logger, fold pre-check into Run, yield events/errors |
| `internal/transfer/download.go` | remove logger, add orchestrator struct, Run becomes iter.Seq2 method |
| `internal/core/builder.go` | replace log.Printf with emit callback |
| `internal/core/installer.go` | replace log.Printf with emit callback |
| `cmd/satisfy/handle_errors.go` | create |
| `cmd/satisfy/main.go` | remove logger, call handle* functions |
| `cmd/satisfy/parse.go` | replace logger.* with fmt.Fprintf(os.Stderr, ...) |

---

## Verification

1. `go build ./...` — confirms no compile errors and go 1.23 features resolve
2. `go test ./...` — all existing tests pass
3. `go vet ./...` — no vet issues
4. Manual smoke test via `go run ./cmd/satisfy version` to confirm CLI output still works
