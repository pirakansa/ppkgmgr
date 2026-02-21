# GitHub Action Specification

This document describes the `pirakansa/ppkgmgr@v0` GitHub Action interface based on `action.yml`.

## Inputs

| Input | Default | Required | Used when | Description |
|---|---|---|---|---|
| `command` | `dl` | No | Always | Command to run: `dl`, `zstd`, or `dig` |
| `manifest` | - | Conditionally | `command: dl` | Manifest path or HTTP(S) URL |
| `version` | `latest` | No | Always | ppkgmgr version to download before execution |
| `overwrite` | `false` | No | `command: dl` | Passes `--overwrite` to `ppkgmgr dl` |
| `src` | - | Conditionally | `command: zstd` | Source file path |
| `dst` | - | Conditionally | `command: zstd` | Destination path for compressed file |
| `file` | - | Conditionally | `command: dig` | File path for digest computation |
| `mode` | `file` | No | `command: dig` | Digest mode: `file` or `artifact` |
| `format` | `raw` | No | `command: dig` | Output format: `raw` or `yaml` |

## Command-specific requirements

- `dl`
  - `manifest` is required.
  - Optional `overwrite: true` enables overwrite without backups.
- `zstd`
  - `src` and `dst` are required.
  - Emits `digest` output.
- `dig`
  - `file` is required.
  - With `mode: artifact`, emits both `digest` and `artifact-digest` outputs.
  - With `format: yaml`, emits `yaml` output.

## Outputs

| Output | Description |
|---|---|
| `digest` | BLAKE3 digest of the output file (`zstd`) or digest result (`dig`) |
| `artifact-digest` | BLAKE3 artifact digest before decoding (`dig --mode artifact`) |
| `yaml` | Manifest-ready YAML snippet (`dig --format yaml`) |

## Notes

- `version: latest` resolves the latest release tag at runtime.
- Unsupported `command` values fail the action.
- The action auto-detects platform and downloads the corresponding release tarball.

## Related docs

- Release workflow example: [release-manifest-workflow.md](./release-manifest-workflow.md)
- Manifest schema: [manifest.md](./manifest.md)
- Artifact authoring: [artifact-authoring.md](./artifact-authoring.md)
