# private-package-manager

`private-package-manager` is a package management tool built with Go. This tool allows you to download files from specified repositories and save them locally.

## Installation

### Quick Install (Linux / macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/pirakansa/ppkgmgr/main/install.sh | bash
```

You can specify a version and installation directory:

```sh
PPKGMGR_VERSION=v0.8.0 PPKGMGR_INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/pirakansa/ppkgmgr/main/install.sh | bash
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
- uses: pirakansa/ppkgmgr@v0
  with:
    manifest: ./path/to/manifest.yml
```

With options:

```yaml
- uses: pirakansa/ppkgmgr@v0
  with:
    manifest: https://example.com/manifest.yml
    version: v0.8.0      # Pin to a specific version (default: latest)
    overwrite: true      # Overwrite existing files without backups
```

For full Action inputs/outputs and command requirements, see:

- [docs/github-action.md](./docs/github-action.md)

#### Creating releases with ppkgmgr-compatible manifests

For release/CI-focused workflows, see:

- [docs/release-manifest-workflow.md](./docs/release-manifest-workflow.md)

## Usage

```sh
$ ppkgmgr dl <path_or_url_to_yaml>  # Execute with a YAML file from disk or an HTTP(S) URL
```

For full command examples and operational details, see:

- [docs/usage.md](./docs/usage.md)

## Documentation

### User guides

- Usage and CLI operations: [docs/usage.md](./docs/usage.md)
- Artifact authoring workflow: [docs/artifact-authoring.md](./docs/artifact-authoring.md)
- Release workflow (GitHub Actions): [docs/release-manifest-workflow.md](./docs/release-manifest-workflow.md)

### Specification references

- Manifest specification (current format `version: 3`): [docs/manifest.md](./docs/manifest.md)
- GitHub Action interface specification: [docs/github-action.md](./docs/github-action.md)
- Reference fixture: [test/data/testdata.yml](./test/data/testdata.yml)
- Documentation index: [docs/README.md](./docs/README.md)

## Development

Recommended for contributors: use VS Code Dev Container or GitHub Codespaces with the `.devcontainer/` image.

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
