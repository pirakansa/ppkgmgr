package cli

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/pirakansa/ppkgmgr/internal/cli/shared"
	"github.com/zeebo/blake3"
	yaml "gopkg.in/yaml.v3"
)

func TestRun_Dig(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "tool.bin")
	content := []byte("digest me please")
	if err := os.WriteFile(target, content, 0o644); err != nil {
		t.Fatalf("failed to write sample file: %v", err)
	}

	hasher := blake3.Sum256(content)
	expected := hex.EncodeToString(hasher[:])

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"util", "dig", target}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stdout.String() != expected+"\n" {
		t.Fatalf("expected stdout %q, got %q", expected+"\n", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_DigYAMLSnippet(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "tool.bin")
	content := []byte("digest yaml please")
	if err := os.WriteFile(target, content, 0o644); err != nil {
		t.Fatalf("failed to write sample file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"util", "dig", "--format", "yaml", target}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var snippet struct {
		Files []struct {
			FileName string `yaml:"file_name"`
			OutDir   string `yaml:"out_dir"`
			Digest   string `yaml:"digest"`
		} `yaml:"files"`
	}
	if err := yaml.Unmarshal(stdout.Bytes(), &snippet); err != nil {
		t.Fatalf("failed to decode yaml: %v", err)
	}
	if len(snippet.Files) != 1 {
		t.Fatalf("expected one file entry, got %d", len(snippet.Files))
	}

	hash := blake3.Sum256(content)
	expectedDigest := hex.EncodeToString(hash[:])
	entry := snippet.Files[0]
	if entry.FileName != filepath.Base(target) {
		t.Fatalf("unexpected file_name: %q", entry.FileName)
	}
	if entry.OutDir != filepath.Dir(target) {
		t.Fatalf("unexpected out_dir: %q", entry.OutDir)
	}
	if entry.Digest != expectedDigest {
		t.Fatalf("unexpected digest: got %q, want %q", entry.Digest, expectedDigest)
	}
}

func TestRun_DigArtifactYAMLSnippet(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "tool.zst")
	original := []byte("artifact digest yaml")

	artifactFile, err := os.Create(artifact)
	if err != nil {
		t.Fatalf("failed to create artifact: %v", err)
	}
	encoder, err := zstd.NewWriter(artifactFile)
	if err != nil {
		artifactFile.Close()
		t.Fatalf("failed to create encoder: %v", err)
	}
	if _, err := encoder.Write(original); err != nil {
		encoder.Close()
		artifactFile.Close()
		t.Fatalf("failed to compress data: %v", err)
	}
	if err := encoder.Close(); err != nil {
		artifactFile.Close()
		t.Fatalf("failed to finalize encoder: %v", err)
	}
	if err := artifactFile.Close(); err != nil {
		t.Fatalf("failed to close artifact: %v", err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"util", "dig", "--mode", "artifact", "--format", "yaml", artifact}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var snippet struct {
		Files []struct {
			FileName       string `yaml:"file_name"`
			OutDir         string `yaml:"out_dir"`
			Digest         string `yaml:"digest"`
			ArtifactDigest string `yaml:"artifact_digest"`
			Encoding       string `yaml:"encoding"`
		} `yaml:"files"`
	}
	if err := yaml.Unmarshal(stdout.Bytes(), &snippet); err != nil {
		t.Fatalf("failed to decode yaml: %v", err)
	}
	if len(snippet.Files) != 1 {
		t.Fatalf("expected one file entry, got %d", len(snippet.Files))
	}

	yamlOutput := stdout.String()
	_, computedArtifactDigest, err := shared.VerifyDigest(artifact, "")
	if err != nil {
		t.Fatalf("failed to recompute artifact digest: %v", err)
	}
	contentHash := blake3.Sum256(original)
	expectedContentDigest := hex.EncodeToString(contentHash[:])

	entry := snippet.Files[0]
	if entry.FileName != filepath.Base(artifact) {
		t.Fatalf("unexpected file_name: %q", entry.FileName)
	}
	if entry.OutDir != filepath.Dir(artifact) {
		t.Fatalf("unexpected out_dir: %q", entry.OutDir)
	}
	if entry.ArtifactDigest != computedArtifactDigest {
		t.Fatalf("unexpected artifact digest: got %q, want %q", entry.ArtifactDigest, computedArtifactDigest)
	}
	if entry.Digest != expectedContentDigest {
		t.Fatalf("unexpected digest: got %q, want %q", entry.Digest, expectedContentDigest)
	}
	if entry.Encoding != "zstd" {
		t.Fatalf("unexpected encoding: %q", entry.Encoding)
	}
	if !strings.Contains(yamlOutput, computedArtifactDigest) {
		t.Fatalf("expected digest to be present in yaml output")
	}
}

func TestRun_DigArtifactRaw(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "tool.zst")
	content := []byte("raw artifact digest")

	artifactFile, err := os.Create(artifact)
	if err != nil {
		t.Fatalf("failed to create artifact: %v", err)
	}
	encoder, err := zstd.NewWriter(artifactFile)
	if err != nil {
		artifactFile.Close()
		t.Fatalf("failed to create encoder: %v", err)
	}
	if _, err := encoder.Write(content); err != nil {
		encoder.Close()
		artifactFile.Close()
		t.Fatalf("failed to compress data: %v", err)
	}
	if err := encoder.Close(); err != nil {
		artifactFile.Close()
		t.Fatalf("failed to finalize encoder: %v", err)
	}
	if err := artifactFile.Close(); err != nil {
		t.Fatalf("failed to close artifact: %v", err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"util", "dig", "--mode", "artifact", artifact}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	artifactDigest := strings.TrimSpace(stdout.String())
	if artifactDigest == "" {
		t.Fatalf("expected artifact digest output")
	}
	_, expectedDigest, err := shared.VerifyDigest(artifact, "")
	if err != nil {
		t.Fatalf("failed to compute expected digest: %v", err)
	}
	if artifactDigest != expectedDigest {
		t.Fatalf("unexpected artifact digest: got %q, want %q", artifactDigest, expectedDigest)
	}
}

func TestRun_DigInvalidMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write sample file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"util", "dig", "--mode", "unknown", path}, &stdout, &stderr, nil)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "invalid mode") {
		t.Fatalf("expected invalid mode message, got %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
}

func TestRun_DigInvalidFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write sample file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"util", "dig", "--format", "unknown", path}, &stdout, &stderr, nil)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "invalid format") {
		t.Fatalf("expected invalid format message, got %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
}

func TestRun_DigRequireArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"util", "dig"}, &stdout, &stderr, nil)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "require file path argument") {
		t.Fatalf("expected argument error, got %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
}

func TestRun_UtilZstd(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "input.txt")
	content := []byte("compress me with zstd")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	dst := filepath.Join(dir, "artifacts", "output.zst")
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"util", "zstd", src, dst}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%q)", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	digestOutput := strings.TrimSpace(stdout.String())
	if digestOutput == "" {
		t.Fatalf("expected digest output, got empty string")
	}

	_, expectedDigest, err := shared.VerifyDigest(dst, "")
	if err != nil {
		t.Fatalf("failed to compute expected digest: %v", err)
	}
	if digestOutput != expectedDigest {
		t.Fatalf("expected digest %q, got %q", expectedDigest, digestOutput)
	}

	compressedData, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read compressed file: %v", err)
	}

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatalf("failed to create decoder: %v", err)
	}
	decompressed, err := decoder.DecodeAll(compressedData, nil)
	decoder.Close()
	if err != nil {
		t.Fatalf("failed to decompress data: %v", err)
	}
	if string(decompressed) != string(content) {
		t.Fatalf("unexpected decompressed content: got %q, want %q", decompressed, content)
	}
}

func TestRun_UtilZstdMissingInput(t *testing.T) {
	tempDir := t.TempDir()
	missingSrc := filepath.Join(tempDir, "missing.txt")
	dst := filepath.Join(tempDir, "output.zst")

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"util", "zstd", missingSrc, dst}, &stdout, &stderr, nil)
	if exitCode != 5 {
		t.Fatalf("expected exit code 5, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "failed to open source") {
		t.Fatalf("expected source error, got %q", stderr.String())
	}
}

func TestRun_UtilZstdSamePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.zst")
	originalContent := []byte("existing data")
	if err := os.WriteFile(path, originalContent, 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"util", "zstd", path, path}, &stdout, &stderr, nil)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "source and destination paths must be different") {
		t.Fatalf("expected identical path error, got %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read original file: %v", err)
	}
	if !bytes.Equal(content, originalContent) {
		t.Fatalf("expected file to remain unchanged; got %q", content)
	}
}

func TestRun_UtilZstdUnwritableDestination(t *testing.T) {
	tempDir := t.TempDir()
	blocked := filepath.Join(tempDir, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("failed to set up blocked path: %v", err)
	}

	src := filepath.Join(tempDir, "input.txt")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	dst := filepath.Join(blocked, "output.zst")
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"util", "zstd", src, dst}, &stdout, &stderr, nil)
	if exitCode != 5 {
		t.Fatalf("expected exit code 5, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "failed to create destination directory") {
		t.Fatalf("expected destination directory error, got %q", stderr.String())
	}
}
