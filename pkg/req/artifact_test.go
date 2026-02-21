package req

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
