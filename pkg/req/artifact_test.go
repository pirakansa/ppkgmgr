package req

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ulikunitz/xz"
)

func TestDecodeArtifact_RequiresOutputPathForNonArchive(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.bin")
	if err := os.WriteFile(source, []byte("data"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	_, err := DecodeArtifact(DecodeArtifactOptions{
		Encoding:   "",
		SourcePath: source,
	})
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "output path is required") {
		t.Fatalf("expected output path error, got %v", err)
	}
}

func TestDecodeArtifact_RequiresOutputDirForArchive(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.tar.gz")
	if err := os.WriteFile(source, createTarGzipForArtifactTests(t, map[string][]byte{"tool": []byte("bin")}), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	_, err := DecodeArtifact(DecodeArtifactOptions{
		Encoding:   "tar+gzip",
		SourcePath: source,
		Extract:    "tool",
	})
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "output directory is required") {
		t.Fatalf("expected output directory error, got %v", err)
	}
}

func TestDecodeArtifact_TarGzipExtractAndRename(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "tool.tar.gz")
	archive := createTarGzipForArtifactTests(t, map[string][]byte{
		"bin/tool": []byte("tool-binary"),
	})
	if err := os.WriteFile(archivePath, archive, 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	outDir := t.TempDir()
	outputPath, err := DecodeArtifact(DecodeArtifactOptions{
		Encoding:   "tar+gzip",
		SourcePath: archivePath,
		OutputDir:  outDir,
		Extract:    "bin/tool",
		Rename:     "tool",
	})
	if err != nil {
		t.Fatalf("DecodeArtifact returned error: %v", err)
	}

	wantPath := filepath.Join(outDir, "tool")
	if outputPath != wantPath {
		t.Fatalf("expected output path %q, got %q", wantPath, outputPath)
	}
	contents, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(contents) != "tool-binary" {
		t.Fatalf("unexpected output content: %q", string(contents))
	}
}

func TestDecodeArtifact_TarXzExtractAllWithSymlink(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "node.tar.xz")
	archive := createTarXzForArtifactTests(t, []tarTestEntry{
		{
			Header: tar.Header{
				Name: "node-v24.13.1-linux-x64/bin/node",
				Mode: 0o755,
				Size: int64(len("node-binary")),
			},
			Content: []byte("node-binary"),
		},
		{
			Header: tar.Header{
				Name:     "node-v24.13.1-linux-x64/bin/corepack",
				Mode:     0o777,
				Typeflag: tar.TypeSymlink,
				Linkname: "../lib/node_modules/corepack/dist/corepack.js",
			},
		},
		{
			Header: tar.Header{
				Name: "node-v24.13.1-linux-x64/lib/node_modules/corepack/dist/corepack.js",
				Mode: 0o644,
				Size: int64(len("corepack")),
			},
			Content: []byte("corepack"),
		},
	})
	if err := os.WriteFile(archivePath, archive, 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	outDir := t.TempDir()
	outputPath, err := DecodeArtifact(DecodeArtifactOptions{
		Encoding:   "tar+xz",
		SourcePath: archivePath,
		OutputDir:  outDir,
	})
	if err != nil {
		t.Fatalf("DecodeArtifact returned error: %v", err)
	}
	if outputPath != "" {
		t.Fatalf("expected empty output path for full extraction, got %q", outputPath)
	}

	linkPath := filepath.Join(outDir, "node-v24.13.1-linux-x64", "bin", "corepack")
	gotTarget, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("read symlink: %v", err)
	}
	if gotTarget != "../lib/node_modules/corepack/dist/corepack.js" {
		t.Fatalf("unexpected symlink target: %q", gotTarget)
	}
}

func createTarGzipForArtifactTests(tb testing.TB, files map[string][]byte) []byte {
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

type tarTestEntry struct {
	Header  tar.Header
	Content []byte
}

func createTarXzForArtifactTests(tb testing.TB, entries []tarTestEntry) []byte {
	tb.Helper()

	var compressed bytes.Buffer
	xzw, err := xz.NewWriter(&compressed)
	if err != nil {
		tb.Fatalf("create xz writer: %v", err)
	}
	tw := tar.NewWriter(xzw)

	for _, entry := range entries {
		header := entry.Header
		if header.Typeflag == 0 {
			header.Typeflag = tar.TypeReg
		}
		if err := tw.WriteHeader(&header); err != nil {
			tb.Fatalf("write tar header: %v", err)
		}
		if header.Typeflag == tar.TypeReg || header.Typeflag == 0 {
			if _, err := tw.Write(entry.Content); err != nil {
				tb.Fatalf("write tar content: %v", err)
			}
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
