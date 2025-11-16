# private-package-manager

`private-package-manager` is a package management tool built with Go. This tool allows you to download files from specified repositories and save them locally.

## Features
- Parse YAML files to retrieve repository information.
- Download files via HTTP.

## Usage

### Command Examples
```sh
$ ppkgmgr help  # Display available subcommands and usage
$ ppkgmgr dl <path_or_url_to_yaml>  # Execute with a YAML file from disk or an HTTP(S) URL
$ ppkgmgr dl -f <path_or_url_to_yaml>  # Overwrite existing files without keeping backups
$ ppkgmgr dl --spider <path_or_url_to_yaml>  # Preview download URLs and paths
$ ppkgmgr repo add <path_or_url_to_yaml>  # Backup the manifest under ~/.ppkgmgr for later use
$ ppkgmgr repo ls  # Show registered manifests stored locally
$ ppkgmgr repo rm <id_or_source>  # Remove a stored manifest by ID or source URL/path
$ ppkgmgr pkg up  # Refresh stored manifests under ~/.ppkgmgr and redownload their files
$ ppkgmgr ver  # Display version information
$ ppkgmgr dig <path_to_file>  # Show the BLAKE3 digest for a file
```

`repo add` keeps a copy of the manifest under `~/.ppkgmgr/manifests` and maintains metadata (including source path/URL and digest) inside `~/.ppkgmgr/registry.json`. Use `repo ls` to inspect saved manifests and `repo rm` when you want to delete an entry. This registry will later be used by commands such as `repo fetch` to detect changes.

`pkg up` reads the stored manifests under `~/.ppkgmgr`, refreshes them from their original sources when possible, and downloads all referenced files so local copies stay up to date. When the refreshed manifest has the same digest as the stored copy, downloads are skipped to avoid unnecessary work.

Running `ppkgmgr dl` without additional flags now preserves any pre-existing files by moving them to `<filename>.bak` (or a numbered variant) before downloading replacements. Supply `-f`/`--force` when you want to skip this backup and overwrite files immediately. The same safeguard applies when `pkg up` notices a digest-protected file has been modified locally or when `repo rm` deletes tracked files—those files are renamed to `.bak` variants so user changes stay recoverable.

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
        out_dir: <output directory for the file>
        rename: <optional new name for the file>
```

### Example YAML File

For an example YAML file, refer to [testdata.yml](./test/data/testdata.yml).

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
