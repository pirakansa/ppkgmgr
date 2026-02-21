# Usage

## Quick Reference

```sh
ppkgmgr help                  # Show available subcommands and usage
ppkgmgr dl <manifest>         # Download files defined in a manifest
ppkgmgr repo add <manifest>   # Register a manifest locally
ppkgmgr pkg up                # Refresh stored manifests and redownload files
ppkgmgr ver                   # Display version information
ppkgmgr util dig <file>       # Compute the BLAKE3 digest of a file
ppkgmgr util zstd <src> <dst> # Compress a file with zstd and print the digest
```

> `<manifest>` accepts a local file path or an HTTP(S) URL.

---

## Commands

### `dl` — Download Files

Downloads files defined in a manifest YAML.

```sh
ppkgmgr dl <manifest>              # Basic download
ppkgmgr dl -o <manifest>           # Overwrite existing files without backups
ppkgmgr dl --spider <manifest>     # Preview download URLs and paths (no actual download)
```

| Flag | Short | Description |
|---|---|---|
| `--overwrite` | `-o` | Overwrite existing files without creating backups |
| `--spider` | — | Print URLs and destination paths only |

### `repo` — Manifest Management

Registers, lists, and removes manifests stored under `~/.ppkgmgr`. Manifest metadata (source path/URL and digest) is maintained in `~/.ppkgmgr/registry.json`.

```sh
ppkgmgr repo add <manifest>        # Register a manifest
ppkgmgr repo ls                    # List registered manifests
ppkgmgr repo rm <id_or_source>     # Remove a manifest by ID or source URL/path
```

### `pkg up` — Bulk Update

Refreshes stored manifests from their original sources and downloads all referenced files to keep local copies up to date.

```sh
ppkgmgr pkg up                     # Update (skip when digest matches)
ppkgmgr pkg up --redownload        # Force download even when digest matches
```

| Flag | Short | Description |
|---|---|---|
| `--redownload` | `-r` | Download even if the manifest digest matches (backup behavior is preserved) |

### `ver` — Version Information

```sh
ppkgmgr ver
```

### `util` — Utilities

#### `util dig` — Digest Computation

Computes the BLAKE3 digest of a file.

```sh
ppkgmgr util dig <file>                                  # Print the digest
ppkgmgr util dig --format yaml <file>                    # Emit a manifest-ready YAML snippet
ppkgmgr util dig --mode artifact --format yaml <file>    # Emit digest + artifact_digest for a compressed artifact
```

| Flag | Default | Description |
|---|---|---|
| `--format` | `raw` | Output format: `raw` (hash only) / `yaml` (YAML snippet) |
| `--mode` | `file` | Input mode: `file` (regular file) / `artifact` (compressed artifact) |

#### `util zstd` — Zstd Compression

Compresses a file with zstd and prints the resulting BLAKE3 digest.

```sh
ppkgmgr util zstd <src> <dst>
```

---

## Configuration

### Data Directory (`PPKGMGR_HOME`)

The internal state directory defaults to `~/.ppkgmgr`. Set the `PPKGMGR_HOME` environment variable to relocate it.

```sh
export PPKGMGR_HOME=/path/to/custom/dir
```

This affects commands that read or write registry data and stored manifests, such as `repo add`, `repo ls`, `repo rm`, and `pkg up`.

---

## Backup Behavior

By default, `ppkgmgr dl` renames pre-existing files to `<filename>.bak` (or a numbered variant) before downloading replacements. Pass `-o` / `--overwrite` to skip backups and overwrite immediately.

The same backup safeguard also applies in the following scenarios:

- **`pkg up`**: When a digest-protected file has been modified locally (including when `--redownload` is used)
- **`repo rm`**: When deleting tracked files

In all cases, files are renamed to `.bak` variants so that user changes remain recoverable.

> **Note:** Files without digests are always overwritten because the tool cannot detect whether a user modification occurred.
