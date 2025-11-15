# private-package-manager

`private-package-manager` is a package management tool built with Go. This tool allows you to download files from specified repositories and save them locally.

## Features
- Parse YAML files to retrieve repository information.
- Download files via HTTP.
- Use `spider` mode to preview download URLs and paths.

## Usage

### Command Examples
```sh
$ ppkgmgr help  # Display available subcommands and usage
$ ppkgmgr dl <path_or_url_to_yaml>  # Execute with a YAML file from disk or an HTTP(S) URL
$ ppkgmgr dl --spider <path_or_url_to_yaml>  # Preview download URLs and paths
$ ppkgmgr pkg add <path_or_url_to_yaml>  # Backup the manifest under ~/.ppkgmgr for later use
$ ppkgmgr ver  # Display version information
```

`pkg add` keeps a copy of the manifest under `~/.ppkgmgr/manifests` and maintains metadata (including source path/URL and digest) inside `~/.ppkgmgr/registry.json`. This registry will later be used by commands such as `pkg fetch` to detect changes.

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
        out_dir: <output directory for the file>
        rename: <optional new name for the file>
```

### Example YAML File

For an example YAML file, refer to [testdata.yml](./test/data/testdata.yml).

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
