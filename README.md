# satisfy

`satisfy` is a lightweight package and version manager built around Google Cloud
Storage (GCS). It uploads, downloads, and verifies versioned package archives,
managing a JSON manifest and a compressed archive (zstd, gzip, or zip) for each
version. Local integrity checks let it skip re-downloading content that is
already present and correct.

It is typically used to publish and consume versioned data packages: a build
job uploads a new version, and downstream pipelines or services download a
specific version, the `latest` version, or — new in this release — a named
**tag**.

## Installation

Build from source (requires Go; see `go.mod` for the required version):

```bash
make compile          # produces ./workspace/satisfy
# or
go build -o satisfy ./cmd/satisfy
```

Print the version of a built binary:

```bash
satisfy version
```

## Authentication

`satisfy` authenticates to GCS with Google service-account credentials. Point
`GOOGLE_APPLICATION_CREDENTIALS` at a service-account key file, or rely on the
environment's ambient credentials. (Uploads and tag modifications additionally
support fetching credentials from Vault via `VAULT_ADDR` / `VAULT_TOKEN`.)

## How packages are stored

For a package named `cat-sound-data`, the remote layout is:

```
/cat-sound-data/manifest.json              <- root manifest (latest + tags)
/cat-sound-data/2026.01.A/manifest.json    <- per-version manifest
/cat-sound-data/2026.01.A/archive          <- per-version compressed archive
/cat-sound-data/2026.01.B/manifest.json
/cat-sound-data/2026.01.B/archive
```

The **root manifest** is a copy of the most recently uploaded version's
manifest. It is what the keyword `latest` resolves to, and it is the only place
**tags** are stored. A per-version manifest never contains tags.

A manifest looks like this:

```json
{
  "name": "cat-sound-data",
  "version": "2026.01.B",
  "archive": {
    "filename": "archive",
    "size": 2506300011,
    "md5": "34GMLE2LaQI1QHKoHJFRbQ==",
    "contents": [
      { "path": "meows.txt", "size": 16087014133, "md5": "4ksM3tWryJom2O8XrU1l5A==" },
      { "path": "purrs.txt", "size": 39014497,    "md5": "E4LTd7kXWcP6kdAvTy9sHg==" }
    ],
    "compression": "zstd"
  }
}
```

The root manifest additionally carries a `tags` array (see
[Tags](#tags)).

## Commands

| Command | Purpose |
|---------|---------|
| *(none)* | Download dependencies (the default action). |
| `upload` | Build and upload a package version, optionally applying tags. |
| `check` | Report whether a version has already been uploaded. |
| `latest` | Print the latest published version of a package. |
| `tags list` | List a package's tags and the versions they point to. |
| `tags modify` | Add, update, or delete tags without re-uploading. |
| `version` | Print the `satisfy` tool version. |

Commands that take a JSON request read it from a file via `-json <path>`, or
from standard input when `-json` is omitted (the default, `_STDIN_`).

---

### Download (default)

Running `satisfy` with no subcommand downloads the dependencies described by a
listing. Each dependency's `package_version` may be a literal version, the
keyword `latest`, or a tag name.

```bash
satisfy -json dependencies.json
# or
cat dependencies.json | satisfy
```

```json
{
  "dependencies": [
    {
      "package_name": "cat-sound-data",
      "package_version": "latest",
      "remote_address": "gcs://cats-data-dev",
      "local_directory": "."
    }
  ]
}
```

- `remote_address` is the bucket (and optional path prefix), e.g.
  `gcs://bucket/releases`.
- `local_directory` is where the archive is extracted. `~/`, `$HOME`, and
  `${HOME}` prefixes are expanded.
- An optional top-level `"credentials"` field selects a credential profile.

`satisfy` writes a local `manifest_<package>.json` next to the extracted files
and uses it to skip downloads when the installed version already matches and
passes its integrity check.

Flags:

| Flag | Default | Meaning |
|------|---------|---------|
| `-json` | `_STDIN_` | Path to the dependency listing, or `_STDIN_`. |
| `-max-retry` | `5` | Retry attempts for downloads. |
| `-quick` | `true` | When `false`, fully re-hash file contents during verification instead of the quick listing check. |
| `-progress` | `true` | Show extraction progress. |

Package names passed as non-flag arguments filter the listing, e.g.
`satisfy -json deps.json cat-sound-data` installs only that dependency.

---

### Upload

Builds an archive from a source path, computes its manifest, and uploads both
the versioned manifest/archive and the updated root manifest.

```bash
satisfy upload -json upload.json
```

```json
{
  "package_name": "cat-sound-data",
  "package_version": "2026.02.B",
  "compression_algorithm": "zstd",
  "compression_level": 9,
  "remote_address": "gcs://cats-data-dev",
  "tags": ["experimental"],
  "source_path": "/data/satisfy/cats"
}
```

- `compression_algorithm` is one of `zstd`, `gzip`, or `zip`.
- Provide exactly one source: `source_path`, `source_directory`, or
  `source_file`.
- `tags` is optional; see [Tagging during upload](#tagging-during-upload).

Flags:

| Flag | Default | Meaning |
|------|---------|---------|
| `-json` | `_STDIN_` | Path to the upload request, or `_STDIN_`. |
| `-max-retry` | `5` | Retry attempts for uploads. |
| `-overwrite` | `false` | Upload even if the version already exists remotely. |
| `-progress` | `true` | Show archiving progress. |

Exit codes: `0` success, `1` failure, `2` the version was already uploaded
(unless `-overwrite` is set).

---

### Check

Reports whether `package@version` (from an upload-style request) already exists
remotely. Useful as a guard before an expensive upload. Exits `2` if the
version is already present.

```bash
satisfy check -json upload.json
```

---

### Latest

Prints the latest published version of a package (the version recorded in the
root manifest) to stdout.

```bash
satisfy latest -bucket cats-data-dev -package cat-sound-data
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-bucket` | — | GCS bucket name (required, bare name — no scheme or slashes). |
| `-path` | — | Optional path prefix within the bucket. |
| `-package` | — | Package name (required). |
| `-max-retry` | `5` | Retry attempts. |

---

## Tags

A **tag** is a named pointer to a specific version that moves only when you move
it. Unlike `latest`, which always tracks the most recent upload, a tag such as
`stable` or `release` lets downstream consumers depend on a stable name and be
promoted on your schedule — ideal for CI/CD, where a live service can always
pull the `release` tag without knowing the exact version.

Tags live only in the root manifest:

```json
{
  "name": "cat-sound-data",
  "version": "2026.02.A",
  "archive": { "...": "..." },
  "tags": [
    { "name": "experimental", "version": "2026.01.B" },
    { "name": "stable",       "version": "2026.01.A" }
  ]
}
```

Rules:

- `latest` is a reserved name and cannot be used as a tag (the download keyword
  always wins). Tag names may not be blank.
- The tag list is kept sorted by name for stable, reviewable diffs.

### Downloading by tag

Put a tag name where you would put a version:

```json
{
  "dependencies": [
    {
      "package_name": "cat-sound-data",
      "package_version": "stable",
      "remote_address": "gcs://cats-data-dev",
      "local_directory": "."
    }
  ]
}
```

Resolution is **version-first**: `satisfy` first checks for a manifest at the
literal version path (a cheap `HEAD` request) and only falls back to resolving
the string as a tag when no such version exists. So a real version always
shadows a tag of the same name, and ordinary pinned-version downloads behave
exactly as before with no extra cost. If a tag resolves to the version already
installed locally, the download is skipped. If the string matches neither a
version nor a tag, the download fails *before* removing any local files.

### Tagging during upload

Add a `tags` array to an upload request to point those tag names at the version
being uploaded:

```json
{
  "package_name": "cat-sound-data",
  "package_version": "2026.02.B",
  "compression_algorithm": "zstd",
  "compression_level": 9,
  "remote_address": "gcs://cats-data-dev",
  "tags": ["experimental"],
  "source_path": "/data/satisfy/cats"
}
```

Existing tags are **preserved**: uploading with `"tags": ["experimental"]` moves
only `experimental` to the new version and leaves an unrelated `stable` tag
where it was. An upload with no `tags` field leaves all tags untouched.

### Listing tags

```bash
satisfy tags list -bucket cats-data-dev -package cat-sound-data
```

Prints one `name<TAB>version` line per tag, sorted by name:

```
experimental	2026.01.B
stable	2026.01.A
```

A package with no tags prints nothing and exits `0`. Add `-json` for a JSON
array instead (an empty list prints `[]`):

```bash
satisfy tags list -bucket cats-data-dev -package cat-sound-data -json
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-bucket` | — | GCS bucket name (required, bare name). |
| `-path` | — | Optional path prefix within the bucket. |
| `-package` | — | Package name (required). |
| `-json` | `false` | Emit a JSON array instead of tab-separated lines. |
| `-max-retry` | `5` | Retry attempts. |

### Modifying tags

Add, update, or delete tags on an already-uploaded package without re-uploading
any data:

```bash
satisfy tags modify -json modify-tags.json
```

```json
{
  "package_name": "cat-sound-data",
  "remote_address": "gcs://cats-data-dev",
  "add": [
    { "name": "stable",         "version": "2026.02.A" },
    { "name": "whiskers-favorite", "version": "2026.01.B" }
  ],
  "delete": [
    { "name": "active-build-test" }
  ]
}
```

- **add** creates a tag, or updates it to the given version if it already
  exists.
- **delete** removes a tag by name (any `version` is ignored). Deleting a tag
  that does not exist is a no-op, not an error.
- Before writing anything, `satisfy` verifies that every version named in `add`
  actually exists remotely. If any target is missing, the whole request fails
  and **nothing is written** — no partial updates, no dangling tags.

| Flag | Default | Meaning |
|------|---------|---------|
| `-json` | `_STDIN_` | Path to the modification request, or `_STDIN_`. |
| `-max-retry` | `5` | Retry attempts. |

---

## Building and testing

```bash
make test        # format the code, then run the tests
make coverage    # run the tests with the race detector
make compile     # build the binary into ./workspace/satisfy
```

---

#### SMARTY DISCLAIMER: Subject to the terms of the associated license agreement, this software is freely available for your use. This software is FREE, AS IN PUPPIES, and is a gift. Enjoy your new responsibility. This means that while we may consider enhancement requests, we may or may not choose to entertain requests at our sole and absolute discretion.
</content>
