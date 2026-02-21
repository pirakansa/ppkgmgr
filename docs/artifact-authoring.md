# Authoring Artifacts for Manifests

This guide shows how to publish compressed artifacts and generate manifest-ready digest snippets.

## zstd artifact workflow

1. Prepare your file (for example, a built binary).
2. Compress it:

```sh
ppkgmgr util zstd /path/to/input /path/to/output.zst
```

This writes the compressed artifact and prints its BLAKE3 digest.

3. Generate manifest YAML snippet for artifact mode:

```sh
ppkgmgr util dig --mode artifact --format yaml /path/to/output.zst
```

Example output snippet:

```yaml
files:
  - file_name: output.zst
    out_dir: /path/to
    digest: 111111...
    artifact_digest: 222222...
    encoding: zstd
```

4. Adjust `file_name`, `out_dir`, and optionally `rename` for your manifest.

## Uncompressed file workflow

For plain files (no artifact encoding), generate digest-only YAML:

```sh
ppkgmgr util dig --format yaml /path/to/file
```

## Notes

- Use `artifact_digest` to verify the downloaded blob before decoding/extracting.
- Use `digest` to verify the final output after decoding/extracting.
- See [manifest.md](./manifest.md) for full manifest field reference.
