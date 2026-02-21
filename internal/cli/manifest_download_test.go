package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/pirakansa/ppkgmgr/internal/cli/manifest"
	"github.com/pirakansa/ppkgmgr/internal/data"
	"github.com/ulikunitz/xz"
	"github.com/zeebo/blake3"
)

func TestDownloadManifestFiles_SuccessWithEncodingAndDigests(t *testing.T) {
	contents := []byte("hello download")
	compressed := compressZstd(t, contents)
	artifactDigest := hashHex(compressed)
	decodedDigest := hashHex(contents)

	outDir := t.TempDir()
	fd := data.FileData{
		Repo: []data.Repositories{
			{
				Url: "https://example.com",
				Files: []data.File{
					{
						FileName:       "artifact.zst",
						Encoding:       "zstd",
						ArtifactDigest: artifactDigest,
						Digest:         decodedDigest,
						OutDir:         outDir,
						Rename:         "artifact.txt",
					},
				},
			},
		},
	}

	downloader := func(_, path string) (int64, error) {
		if err := os.WriteFile(path, compressed, 0o644); err != nil {
			return 0, err
		}
		return int64(len(compressed)), nil
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if err := manifest.DownloadFiles(fd, downloader, stdout, stderr, false, true, false); err != nil {
		t.Fatalf("downloadManifestFiles returned error: %v (stderr: %s)", err, stderr.String())
	}

	outputPath := filepath.Join(outDir, "artifact.txt")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if !bytes.Equal(data, contents) {
		t.Fatalf("unexpected decoded contents: %q", data)
	}
}

func TestDownloadManifestFiles_ArtifactDigestMismatch(t *testing.T) {
	contents := []byte("hello mismatch")
	compressed := compressZstd(t, contents)
	outDir := t.TempDir()
	fd := data.FileData{
		Repo: []data.Repositories{
			{
				Url: "https://example.com",
				Files: []data.File{
					{
						FileName:       "artifact.zst",
						Encoding:       "zstd",
						ArtifactDigest: strings.Repeat("0", 64),
						Digest:         hashHex(contents),
						OutDir:         outDir,
					},
				},
			},
		},
	}

	downloader := func(_, path string) (int64, error) {
		if err := os.WriteFile(path, compressed, 0o644); err != nil {
			return 0, err
		}
		return int64(len(compressed)), nil
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	err := manifest.DownloadFiles(fd, downloader, stdout, stderr, false, true, false)
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	var cliErr cliError
	if !errors.As(err, &cliErr) || cliErr.Code != 4 {
		t.Fatalf("expected cli error code 4, got %v", err)
	}
	if !strings.Contains(stderr.String(), "artifact digest mismatch") {
		t.Fatalf("expected artifact digest mismatch message, got %q", stderr.String())
	}
}

func TestDownloadManifestFiles_DigestMismatch(t *testing.T) {
	contents := []byte("hello digest")
	compressed := compressZstd(t, contents)
	outDir := t.TempDir()
	fd := data.FileData{
		Repo: []data.Repositories{
			{
				Url: "https://example.com",
				Files: []data.File{
					{
						FileName:       "artifact.zst",
						Encoding:       "zstd",
						ArtifactDigest: hashHex(compressed),
						Digest:         strings.Repeat("1", 64),
						OutDir:         outDir,
					},
				},
			},
		},
	}

	downloader := func(_, path string) (int64, error) {
		if err := os.WriteFile(path, compressed, 0o644); err != nil {
			return 0, err
		}
		return int64(len(compressed)), nil
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	err := manifest.DownloadFiles(fd, downloader, stdout, stderr, false, true, false)
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	var cliErr cliError
	if !errors.As(err, &cliErr) || cliErr.Code != 4 {
		t.Fatalf("expected cli error code 4, got %v", err)
	}
	if !strings.Contains(stderr.String(), "digest mismatch") {
		t.Fatalf("expected digest mismatch message, got %q", stderr.String())
	}

	outputPath := filepath.Join(outDir, "artifact.zst")
	if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected %s to be removed, got stat err = %v", outputPath, statErr)
	}
}

func TestDownloadManifestFiles_UnsupportedEncoding(t *testing.T) {
	outDir := t.TempDir()
	fd := data.FileData{
		Repo: []data.Repositories{
			{
				Url: "https://example.com",
				Files: []data.File{
					{
						FileName:       "artifact.bin",
						Encoding:       "unknown",
						ArtifactDigest: hashHex([]byte("data")),
						OutDir:         outDir,
					},
				},
			},
		},
	}

	downloader := func(_, path string) (int64, error) {
		if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			return 0, err
		}
		return 4, nil
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	err := manifest.DownloadFiles(fd, downloader, stdout, stderr, false, true, false)
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	var cliErr cliError
	if !errors.As(err, &cliErr) || cliErr.Code != 4 {
		t.Fatalf("expected cli error code 4, got %v", err)
	}
	if !strings.Contains(stderr.String(), "unsupported encoding") {
		t.Fatalf("expected unsupported encoding message, got %q", stderr.String())
	}
}

func TestDownloadManifestFiles_TarGzipExtractWithRenameAndMode(t *testing.T) {
	archive := createTarGzip(t, map[string][]byte{
		"codex-x86_64-unknown-linux-musl": []byte("codex-binary"),
	})

	outDir := t.TempDir()
	fd := data.FileData{
		Repo: []data.Repositories{
			{
				Url: "https://example.com",
				Files: []data.File{
					{
						FileName: "codex.tar.gz",
						Encoding: "tar+gzip",
						Extract:  "codex-x86_64-unknown-linux-musl",
						Rename:   "codex",
						OutDir:   outDir,
						Mode:     "0755",
					},
				},
			},
		},
	}

	downloader := func(_, path string) (int64, error) {
		if err := os.WriteFile(path, archive, 0o644); err != nil {
			return 0, err
		}
		return int64(len(archive)), nil
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if err := manifest.DownloadFiles(fd, downloader, stdout, stderr, false, true, false); err != nil {
		t.Fatalf("downloadManifestFiles returned error: %v (stderr: %s)", err, stderr.String())
	}

	outPath := filepath.Join(outDir, "codex")
	contents, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if string(contents) != "codex-binary" {
		t.Fatalf("unexpected extracted content: %q", string(contents))
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("failed to stat extracted file: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("expected mode 0755, got %o", info.Mode().Perm())
	}
}

func TestDownloadManifestFiles_TarXzExtractAllAndSymlink(t *testing.T) {
	archive := createTarXz(t, map[string][]byte{
		"node-v24.13.1-linux-x64/bin/node": []byte("node-binary"),
		"node-v24.13.1-linux-x64/lib/a.js": []byte("console.log('ok')"),
	})

	root := t.TempDir()
	outDir := filepath.Join(root, "lib")
	linkPath := filepath.Join(root, "node")

	fd := data.FileData{
		Repo: []data.Repositories{
			{
				Url: "https://example.com",
				Files: []data.File{
					{
						FileName: "node.tar.xz",
						Encoding: "tar+xz",
						OutDir:   outDir,
						Symlink: &data.SymlinkConfig{
							Link:   linkPath,
							Target: "node-v24.13.1-linux-x64",
						},
					},
				},
			},
		},
	}

	downloader := func(_, path string) (int64, error) {
		if err := os.WriteFile(path, archive, 0o644); err != nil {
			return 0, err
		}
		return int64(len(archive)), nil
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if err := manifest.DownloadFiles(fd, downloader, stdout, stderr, false, true, false); err != nil {
		t.Fatalf("downloadManifestFiles returned error: %v (stderr: %s)", err, stderr.String())
	}

	nodePath := filepath.Join(outDir, "node-v24.13.1-linux-x64", "bin", "node")
	if _, err := os.Stat(nodePath); err != nil {
		t.Fatalf("expected extracted tree at %s: %v", nodePath, err)
	}

	resolvedTarget, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("expected symlink %s: %v", linkPath, err)
	}
	if resolvedTarget != "node-v24.13.1-linux-x64" {
		t.Fatalf("expected symlink target %q, got %q", "node-v24.13.1-linux-x64", resolvedTarget)
	}
}

func compressZstd(tb testing.TB, contents []byte) []byte {
	tb.Helper()
	var buf bytes.Buffer
	encoder, err := zstd.NewWriter(&buf)
	if err != nil {
		tb.Fatalf("create encoder: %v", err)
	}
	if _, err := encoder.Write(contents); err != nil {
		tb.Fatalf("compress contents: %v", err)
	}
	encoder.Close()
	return buf.Bytes()
}

func hashHex(data []byte) string {
	sum := blake3.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

func createTarGzip(tb testing.TB, files map[string][]byte) []byte {
	tb.Helper()

	var compressed bytes.Buffer
	gzw := gzip.NewWriter(&compressed)
	tw := tar.NewWriter(gzw)
	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(header); err != nil {
			tb.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write(content); err != nil {
			tb.Fatalf("write tar content: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		tb.Fatalf("close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		tb.Fatalf("close gzip writer: %v", err)
	}

	return compressed.Bytes()
}

func createTarXz(tb testing.TB, files map[string][]byte) []byte {
	tb.Helper()

	var compressed bytes.Buffer
	xzw, err := xz.NewWriter(&compressed)
	if err != nil {
		tb.Fatalf("create xz writer: %v", err)
	}
	tw := tar.NewWriter(xzw)
	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(header); err != nil {
			tb.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write(content); err != nil {
			tb.Fatalf("write tar content: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		tb.Fatalf("close tar writer: %v", err)
	}
	if err := xzw.Close(); err != nil {
		tb.Fatalf("close xz writer: %v", err)
	}

	return compressed.Bytes()
}
