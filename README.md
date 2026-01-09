# private-package-manager

`private-package-manager` is a package management tool built with Go. This tool allows you to download files from specified repositories and save them locally.

## Features
- Parse YAML files to retrieve repository information.
- Download files via HTTP.

## Installation

### Quick Install (Linux / macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/pirakansa/ppkgmgr/main/install.sh | bash
```

You can specify a version and installation directory:

```sh
PPKGMGR_VERSION=v0.7.0 PPKGMGR_INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/pirakansa/ppkgmgr/main/install.sh | bash
```

### Using `go install`

If you have Go installed:

```sh
go install github.com/pirakansa/ppkgmgr/cmd/ppkgmgr@latest
```

### Manual Download

Download prebuilt binaries from the [Releases](https://github.com/pirakansa/ppkgmgr/releases) page.

Available platforms:
- Linux (amd64, arm, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

### GitHub Actions

Use ppkgmgr directly in your workflows:

```yaml
# Download files from a manifest
- uses: pirakansa/ppkgmgr@v1
  with:
    manifest: ./path/to/manifest.yml
```

With options:

```yaml
- uses: pirakansa/ppkgmgr@v1
  with:
    manifest: https://example.com/manifest.yml
    version: v0.7.0      # Pin to a specific version (default: latest)
    overwrite: true      # Overwrite existing files without backups
```

#### Creating releases with ppkgmgr-compatible manifests

Compress build artifacts and generate manifest snippets in your release workflow:

```yaml
# Compress a binary with zstd
- uses: pirakansa/ppkgmgr@v1
  id: compress
  with:
    command: zstd
    src: ./bin/myapp
    dst: ./release/myapp.zst

# Generate a YAML snippet for the compressed artifact
- uses: pirakansa/ppkgmgr@v1
  id: manifest
  with:
    command: dig
    file: ./release/myapp.zst
    mode: artifact
    format: yaml

# Use the outputs
- run: |
    echo "Digest: ${{ steps.compress.outputs.digest }}"
    echo "YAML snippet:"
    echo "${{ steps.manifest.outputs.yaml }}"
```

Available outputs:
- `digest` - BLAKE3 digest of the file
- `artifact-digest` - BLAKE3 digest of the artifact before decoding (for `dig --mode artifact`)
- `yaml` - Generated YAML snippet (for `dig --format yaml`)

## Usage

### Command Examples
```sh
$ ppkgmgr help  # Display available subcommands and usage
$ ppkgmgr dl <path_or_url_to_yaml>  # Execute with a YAML file from disk or an HTTP(S) URL
$ ppkgmgr dl --overwrite <path_or_url_to_yaml>  # Overwrite existing files without keeping backups
$ ppkgmgr dl --spider <path_or_url_to_yaml>  # Preview download URLs and paths
$ ppkgmgr repo add <path_or_url_to_yaml>  # Backup the manifest under ~/.ppkgmgr for later use
$ ppkgmgr repo ls  # Show registered manifests stored locally
$ ppkgmgr repo rm <id_or_source>  # Remove a stored manifest by ID or source URL/path
$ ppkgmgr pkg up  # Refresh stored manifests under ~/.ppkgmgr and redownload their files
$ ppkgmgr pkg up --redownload  # Refresh and download even when manifest digests match (backups still apply when possible)
$ ppkgmgr ver  # Display version information
$ ppkgmgr util dig <path_to_file>  # Show the BLAKE3 digest for a file
$ ppkgmgr util dig --format yaml <path_to_file>  # Emit a manifest-ready YAML snippet for a file's digest
$ ppkgmgr util dig --mode artifact --format yaml <path_to_artifact>  # Emit digest + artifact_digest for a compressed artifact
$ ppkgmgr util zstd <src> <dst>  # Compress a file with zstd and print the resulting digest
```

`repo add` keeps a copy of the manifest under `~/.ppkgmgr/manifests` and maintains metadata (including source path/URL and digest) inside `~/.ppkgmgr/registry.json`. Use `repo ls` to inspect saved manifests and `repo rm` when you want to delete an entry. This registry will later be used by commands such as `repo fetch` to detect changes.

`pkg up` reads the stored manifests under `~/.ppkgmgr`, refreshes them from their original sources when possible, and downloads all referenced files so local copies stay up to date. When the refreshed manifest has the same digest as the stored copy, downloads are skipped to avoid unnecessary work. Pass `--redownload` when you want to bypass the digest check and download anyway; this does **not** disable the backup behavior described below.

Set the `PPKGMGR_HOME` environment variable when you need to relocate the internal state directory (defaults to `~/.ppkgmgr`). This applies to commands such as `repo add`, `repo ls`, `repo rm`, and `pkg up` that read or write registry data and stored manifests.

Running `ppkgmgr dl` without additional flags now preserves any pre-existing files by moving them to `<filename>.bak` (or a numbered variant) before downloading replacements. Supply `-o`/`--overwrite` when you want to skip this backup and overwrite files immediately. The same safeguard applies when `pkg up` notices a digest-protected file has been modified locally (even if `--redownload` is used) or when `repo rm` deletes tracked files—those files are renamed to `.bak` variants so user changes stay recoverable. Files without digests are always overwritten because the tool cannot determine whether a user modification occurred, so keep backups yourself when working with digestless entries.

### Authoring manifests for compressed artifacts

Use `ppkgmgr util zstd` to produce zstd-compressed artifacts and capture their digests for manifest entries:

1. Prepare the file you want to publish (for example, a binary build output).
2. Run `ppkgmgr util zstd /path/to/input /path/to/output.zst`. The command creates parent directories as needed, writes the compressed file, and prints its BLAKE3 digest to stdout.
3. Run `ppkgmgr util dig --mode artifact --format yaml /path/to/output.zst` to generate a manifest snippet that includes both the `artifact_digest` (for the compressed blob) and the decoded file `digest`:

   ```yaml
   files:
     - file_name: output.zst
       out_dir: /path/to
       digest: 111111...
       artifact_digest: 222222...
       encoding: zstd
   ```

4. Adjust `file_name`, `out_dir`, or add `rename` as needed before adding the snippet to your manifest.
5. For uncompressed files, run `ppkgmgr util dig --format yaml /path/to/file` to emit a digest-only snippet suitable for a `files` entry.

## YAML Files

The tool relies on YAML files to define the repositories and files to be downloaded. The structure of the YAML file is as follows:

```yaml
version: 2

repositories:
  -
    _comment: <description of the repository>
    url: <base URL of the repository>
    files:
      -
        file_name: <name of the file to download>
        digest: <optional BLAKE3 digest to verify the file>
        artifact_digest: <optional BLAKE3 digest of the downloaded artifact before decoding>
        encoding: <optional encoding name for downloaded artifacts (e.g., zstd)>
        out_dir: <output directory for the file>
        rename: <optional new name for the file>
```

### Example YAML File

For an example YAML file, refer to [testdata.yml](./test/data/testdata.yml).

When `encoding` is provided, downloads are written to a temporary path, validated with `artifact_digest` when present, decoded into the final `rename`/`file_name` destination, and verified again with `digest`.

You can embed environment variables into `out_dir`. For example, `out_dir: $HOME/.local/bin` downloads files under `~/.local/bin` in your home directory.

## Development

### Build
Build the project using:
```sh
$ make build
```

### Test
Run tests using:
```sh
$ make test
```

### Release
Create release binaries using:
```sh
$ make release
```
