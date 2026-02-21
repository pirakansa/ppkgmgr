# Manifest Reference

Current `ppkgmgr` manifest format: `version: 3`

If `version` is omitted, `ppkgmgr` treats it as `3`.

## Schema

```yaml
version: 3

repositories:
  -
    _comment: <description of the repository>
    url: <base URL of the repository>
    files:
      -
        file_name: <name of the file to download>
        digest: <optional BLAKE3 digest to verify final output>
        artifact_digest: <optional BLAKE3 digest to verify downloaded artifact before decoding/extraction>
        encoding: <optional encoding: zstd | tar+gzip | tar+xz>
        extract: <optional archive path to extract>
        out_dir: <output directory>
        rename: <optional output name>
        mode: <optional octal file mode string, e.g. "0755">
        symlink:
          link: <required when symlink is set>
          target: <required when symlink is set>
```

## Processing flow

When `encoding` is provided, downloads are written to a temporary file, validated with `artifact_digest` (if present), decoded/extracted to the destination, then validated again with `digest` (if present).

If extraction produces multiple files (for example full-archive extraction), `digest` cannot be applied to a single output file path. In that case, prefer `artifact_digest` for integrity verification of the downloaded artifact.

## Supported encodings

- `zstd`: decode to a single output file
- `tar+gzip`: extract `.tar.gz`
- `tar+xz`: extract `.tar.xz`

## `extract` behavior (archive encodings)

- `extract` specified (example: `bin/tool`): extracts only that file/directory
- `extract` omitted or `"."`: extracts entire archive contents into `out_dir`

## `rename` behavior

- Effective for single-output cases:
  - normal file decode
  - archive extraction with explicit `extract`
- Ignored when extracting an entire archive

## `mode` behavior

- Applied for single-output results (for example executable binaries)
- Ignored for full-archive extraction

## `symlink` behavior

- Created after successful processing
- Existing path at `symlink.link` is replaced

## Environment variables

Environment variables are expanded in `out_dir` (and symlink paths/targets). Example:

```yaml
out_dir: $HOME/.local/bin
```

## Example: single binary from tar.gz

```yaml
version: 3

repositories:
  -
    _comment: OpenAI Codex CLI
    url: https://github.com/openai/codex/releases/download/rust-v0.104.0/
    files:
      -
        file_name: codex-x86_64-unknown-linux-musl.tar.gz
        encoding: tar+gzip
        extract: codex-x86_64-unknown-linux-musl
        rename: codex
        out_dir: $HOME/.local/bin
        mode: "0755"
```

## Example: full tree extraction from tar.xz + symlink

```yaml
version: 3

repositories:
  -
    _comment: Node.js
    url: https://nodejs.org/dist/v24.13.1/
    files:
      -
        file_name: node-v24.13.1-linux-x64.tar.xz
        encoding: tar+xz
        out_dir: $HOME/.local/lib
        symlink:
          link: $HOME/.local/lib/node
          target: node-v24.13.1-linux-x64
```
