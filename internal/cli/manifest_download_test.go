package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/pirakansa/ppkgmgr/internal/data"
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
	if err := downloadManifestFiles(fd, downloader, stdout, stderr, false, true, false); err != nil {
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
	err := downloadManifestFiles(fd, downloader, stdout, stderr, false, true, false)
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	var cliErr cliError
	if !errors.As(err, &cliErr) || cliErr.code != 4 {
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
	err := downloadManifestFiles(fd, downloader, stdout, stderr, false, true, false)
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	var cliErr cliError
	if !errors.As(err, &cliErr) || cliErr.code != 4 {
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
	err := downloadManifestFiles(fd, downloader, stdout, stderr, false, true, false)
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	var cliErr cliError
	if !errors.As(err, &cliErr) || cliErr.code != 4 {
		t.Fatalf("expected cli error code 4, got %v", err)
	}
	if !strings.Contains(stderr.String(), "unsupported encoding") {
		t.Fatalf("expected unsupported encoding message, got %q", stderr.String())
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
