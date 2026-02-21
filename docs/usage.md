# Usage

## Command Examples

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

## Repository and registry behavior

`repo add` keeps a copy of the manifest under `~/.ppkgmgr/manifests` and maintains metadata (including source path/URL and digest) inside `~/.ppkgmgr/registry.json`. Use `repo ls` to inspect saved manifests and `repo rm` when you want to delete an entry.

`pkg up` reads the stored manifests under `~/.ppkgmgr`, refreshes them from their original sources when possible, and downloads all referenced files so local copies stay up to date. When the refreshed manifest has the same digest as the stored copy, downloads are skipped to avoid unnecessary work. Pass `--redownload` when you want to bypass the digest check and download anyway; this does **not** disable backup behavior.

## Data directory

Set the `PPKGMGR_HOME` environment variable when you need to relocate the internal state directory (defaults to `~/.ppkgmgr`). This applies to commands such as `repo add`, `repo ls`, `repo rm`, and `pkg up` that read or write registry data and stored manifests.

## Backup behavior

Running `ppkgmgr dl` without additional flags preserves pre-existing files by moving them to `<filename>.bak` (or a numbered variant) before downloading replacements. Supply `-o`/`--overwrite` when you want to skip this backup and overwrite files immediately.

The same safeguard applies when `pkg up` notices a digest-protected file has been modified locally (even if `--redownload` is used) or when `repo rm` deletes tracked files—those files are renamed to `.bak` variants so user changes stay recoverable.

Files without digests are always overwritten because the tool cannot determine whether a user modification occurred.
