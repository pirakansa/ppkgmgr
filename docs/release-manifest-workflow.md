# Release Manifest Workflow (GitHub Actions)

This guide explains how to create `ppkgmgr`-compatible manifest snippets from build artifacts in CI.

## Goal

Generate compressed artifacts and digest metadata that can be copied into manifest `files` entries.

## Example workflow snippet

```yaml
# Compress a binary with zstd
- uses: pirakansa/ppkgmgr@v0
  id: compress
  with:
    command: zstd
    src: ./bin/myapp
    dst: ./release/myapp.zst

# Generate a YAML snippet for the compressed artifact
- uses: pirakansa/ppkgmgr@v0
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

## Available outputs

- `digest`: BLAKE3 digest of the file
- `artifact-digest`: BLAKE3 digest of the artifact before decoding (`dig --mode artifact`)
- `yaml`: generated YAML snippet (`dig --format yaml`)

## Related docs

- GitHub Action reference: [github-action.md](./github-action.md)
- Manifest reference: [manifest.md](./manifest.md)
- Artifact authoring: [artifact-authoring.md](./artifact-authoring.md)
