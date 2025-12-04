package cli

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/pirakansa/ppkgmgr/internal/registry"
	yaml "gopkg.in/yaml.v3"

	"github.com/zeebo/blake3"
)

func newLocalHTTPServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skip: failed to listen on loopback: %v", err)
	}
	server := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: handler},
	}
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func expectTempDownloadPath(t *testing.T, path string) {
	t.Helper()
	if !strings.HasPrefix(path, os.TempDir()) {
		t.Fatalf("unexpected download path %q", path)
	}
}

func TestDefaultData(t *testing.T) {
	if got := defaultData("", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
	if got := defaultData("value", "fallback"); got != "value" {
		t.Fatalf("expected value, got %q", got)
	}
}

func TestRun_Version(t *testing.T) {
	Version = "1.2.3"
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"ver"}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stdout.String() != "Version : 1.2.3\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_VersionUsesBuildInfo(t *testing.T) {
	origVersion := Version
	Version = defaultVersion
	t.Cleanup(func() { Version = origVersion })

	origReader := buildInfoReader
	buildInfoReader = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v9.9.9"},
		}, true
	}
	t.Cleanup(func() { buildInfoReader = origReader })

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"ver"}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stdout.String() != "Version : v9.9.9\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_RequireSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{}, &stdout, &stderr, nil)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "run 'ppkgmgr help'") {
		t.Fatalf("expected help suggestion, got %q", stderr.String())
	}
}

func TestRun_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"help"}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "Available Commands") {
		t.Fatalf("expected help text, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

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
	exitCode := Run([]string{"dig", target}, &stdout, &stderr, nil)
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
	exitCode := Run([]string{"dig", "--format", "yaml", target}, &stdout, &stderr, nil)
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
	exitCode := Run([]string{"dig", "--mode", "artifact", "--format", "yaml", artifact}, &stdout, &stderr, nil)
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
	_, computedArtifactDigest, err := verifyDigest(artifact, "")
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
	exitCode := Run([]string{"dig", "--mode", "artifact", artifact}, &stdout, &stderr, nil)
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
	_, expectedDigest, err := verifyDigest(artifact, "")
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
	exitCode := Run([]string{"dig", "--mode", "unknown", path}, &stdout, &stderr, nil)
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
	exitCode := Run([]string{"dig", "--format", "unknown", path}, &stdout, &stderr, nil)
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
	exitCode := Run([]string{"dig"}, &stdout, &stderr, nil)
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

	_, expectedDigest, err := verifyDigest(dst, "")
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

func TestRunRepo_RequireSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"repo"}, &stdout, &stderr, nil)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "require repo subcommand") {
		t.Fatalf("expected repo subcommand message, got %q", stderr.String())
	}
}

func TestRunRepoAdd_RequireArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"repo", "add"}, &stdout, &stderr, nil)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "require manifest path or URL argument") {
		t.Fatalf("expected manifest argument message, got %q", stderr.String())
	}
}

func TestRun_RequireManifestArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"dl"}, &stdout, &stderr, nil)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "require manifest path argument") {
		t.Fatalf("expected require manifest message, got %q", stderr.String())
	}
}

func TestRun_PathNotFound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"dl", "missing.yml"}, &stdout, &stderr, nil)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "not found path") {
		t.Fatalf("expected not found path message, got %q", stderr.String())
	}
}

func TestRun_ParseError(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "bad.yml")
	if err := os.WriteFile(badFile, []byte("repositories: ["), 0o644); err != nil {
		t.Fatalf("failed to write bad file: %v", err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"dl", badFile}, &stdout, &stderr, nil)
	if exitCode != 3 {
		t.Fatalf("expected exit code 3, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "failed to parse data") {
		t.Fatalf("expected parse error message, got %q", stderr.String())
	}
}

func TestRun_Spider(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yml")
	content := "repositories:\n  - url: https://example.com\n    files:\n      - file_name: file.txt\n        out_dir: ./out\n"
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"dl", "--spider", yamlPath}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	expected := "https://example.com/file.txt   out/file.txt\n"
	if stdout.String() != expected {
		t.Fatalf("expected %q, got %q", expected, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_SpiderWithExpandedOutDir(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yml")
	content := "repositories:\n  - url: https://example.com\n    files:\n      - file_name: file.txt\n        out_dir: $HOME/.local/bin\n"
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	home := filepath.Join(dir, "home")
	t.Setenv("HOME", home)

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"dl", "--spider", yamlPath}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	wantPath := filepath.Join(home, ".local/bin", "file.txt")
	expected := fmt.Sprintf("https://example.com/file.txt   %s\n", wantPath)
	if stdout.String() != expected {
		t.Fatalf("expected %q, got %q", expected, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_DownloadSuccess(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yml")
	outDir := filepath.Join(dir, "out")
	content := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: file.txt\n        out_dir: %s\n        rename: saved.txt\n", outDir)
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	var called bool
	downloader := func(url, path string) (int64, error) {
		called = true
		if url != "https://example.com/file.txt" {
			t.Fatalf("unexpected url %q", url)
		}
		expectTempDownloadPath(t, path)
		if err := os.WriteFile(path, []byte("downloaded"), 0o644); err != nil {
			return 0, err
		}
		return 123, nil
	}

	exitCode := Run([]string{"dl", yamlPath}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !called {
		t.Fatalf("expected downloader to be called")
	}
	downloadedPath := filepath.Join(outDir, "saved.txt")
	data, err := os.ReadFile(downloadedPath)
	if err != nil {
		t.Fatalf("expected downloaded file to exist: %v", err)
	}
	if string(data) != "downloaded" {
		t.Fatalf("unexpected file contents: %q", string(data))
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_DownloadAbsoluteRename(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yml")
	outDir := filepath.Join(dir, "out")
	content := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: file.txt\n        out_dir: %s\n        rename: /etc/passwd\n", outDir)
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	downloader := func(url, path string) (int64, error) {
		expectTempDownloadPath(t, path)
		return 0, os.WriteFile(path, []byte("renamed"), 0o644)
	}

	exitCode := Run([]string{"dl", yamlPath}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	renamedPath := filepath.Join(outDir, "etc/passwd")
	data, err := os.ReadFile(renamedPath)
	if err != nil {
		t.Fatalf("expected renamed file to exist: %v", err)
	}
	if string(data) != "renamed" {
		t.Fatalf("unexpected renamed contents: %q", string(data))
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_RemoteYAML(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	server := newLocalHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/config.yml" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, "repositories:\n  - url: https://example.com\n    files:\n      - file_name: remote.txt\n        out_dir: %s\n", outDir)
	}))

	var stdout, stderr bytes.Buffer
	var called bool
	downloader := func(url, path string) (int64, error) {
		called = true
		if url != "https://example.com/remote.txt" {
			t.Fatalf("unexpected url %q", url)
		}
		expectTempDownloadPath(t, path)
		return 1, os.WriteFile(path, []byte("remote"), 0o644)
	}

	exitCode := Run([]string{"dl", server.URL + "/config.yml"}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !called {
		t.Fatalf("expected downloader to be called")
	}
	remotePath := filepath.Join(outDir, "remote.txt")
	if data, err := os.ReadFile(remotePath); err != nil || string(data) != "remote" {
		t.Fatalf("unexpected remote output (err=%v, data=%q)", err, string(data))
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_DownloadError(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yml")
	content := "repositories:\n  - url: https://example.com\n    files:\n      - file_name: file.txt\n"
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	downloadErr := errors.New("download failed")
	downloader := func(url, path string) (int64, error) {
		return 0, downloadErr
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"dl", yamlPath}, &stdout, &stderr, downloader)
	if exitCode != 4 {
		t.Fatalf("expected exit code 4, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "failed to download") {
		t.Fatalf("expected download failure message, got %q", stderr.String())
	}
}

func TestRun_DownloadDigestMatch(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yml")
	fileContent := []byte("digest test data")
	hasher := blake3.New()
	hasher.Write(fileContent)
	digest := hex.EncodeToString(hasher.Sum(nil))

	yamlContent := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: file.txt\n        out_dir: %s\n        digest: %s\n", dir, digest)
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	downloader := func(url, path string) (int64, error) {
		if url != "https://example.com/file.txt" {
			t.Fatalf("unexpected url %q", url)
		}
		expectTempDownloadPath(t, path)
		if err := os.WriteFile(path, fileContent, 0o644); err != nil {
			return 0, err
		}
		return int64(len(fileContent)), nil
	}

	exitCode := Run([]string{"dl", yamlPath}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
	outputPath := filepath.Join(dir, "file.txt")
	if data, err := os.ReadFile(outputPath); err != nil || !bytes.Equal(data, fileContent) {
		t.Fatalf("unexpected output file (err=%v, data=%q)", err, string(data))
	}
}

func TestRun_DownloadDigestMismatch(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yml")
	fileContent := []byte("digest mismatch data")
	yamlContent := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: file.txt\n        out_dir: %s\n        digest: %s\n", dir, strings.Repeat("0", 64))
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	downloader := func(url, path string) (int64, error) {
		if url != "https://example.com/file.txt" {
			t.Fatalf("unexpected url %q", url)
		}
		expectTempDownloadPath(t, path)
		if err := os.WriteFile(path, fileContent, 0o644); err != nil {
			return 0, err
		}
		return int64(len(fileContent)), nil
	}

	exitCode := Run([]string{"dl", yamlPath}, &stdout, &stderr, downloader)
	if exitCode != 4 {
		t.Fatalf("expected exit code 4, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "digest mismatch") {
		t.Fatalf("expected digest mismatch message, got %q", stderr.String())
	}
}

func TestRun_DownloadBackupsExistingFile(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	targetPath := filepath.Join(targetDir, "file.txt")
	original := []byte("original data")
	if err := os.WriteFile(targetPath, original, 0o644); err != nil {
		t.Fatalf("failed to seed file: %v", err)
	}

	yamlPath := filepath.Join(dir, "config.yml")
	content := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: file.txt\n        out_dir: %s\n", targetDir)
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	downloader := func(url, path string) (int64, error) {
		if err := os.WriteFile(path, []byte("new data"), 0o644); err != nil {
			return 0, err
		}
		return 8, nil
	}

	exitCode := Run([]string{"dl", yamlPath}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}

	backupPath := targetPath + ".bak"
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("expected backup %s to exist: %v", backupPath, err)
	}
	if data, err := os.ReadFile(backupPath); err != nil || !bytes.Equal(data, original) {
		t.Fatalf("expected backup to contain original data, got %q err=%v", data, err)
	}
	if data, err := os.ReadFile(targetPath); err != nil || string(data) != "new data" {
		t.Fatalf("expected target to be replaced, got %q err=%v", data, err)
	}
	if !strings.Contains(stderr.String(), "backed up") {
		t.Fatalf("expected backup message, got %q", stderr.String())
	}
}

func TestRun_DownloadOverwriteSkipsBackup(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	targetPath := filepath.Join(targetDir, "file.txt")
	if err := os.WriteFile(targetPath, []byte("force data"), 0o644); err != nil {
		t.Fatalf("failed to seed file: %v", err)
	}

	yamlPath := filepath.Join(dir, "config.yml")
	content := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: file.txt\n        out_dir: %s\n", targetDir)
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	downloader := func(url, path string) (int64, error) {
		if err := os.WriteFile(path, []byte("forced overwrite"), 0o644); err != nil {
			return 0, err
		}
		return 16, nil
	}

	exitCode := Run([]string{"dl", "--overwrite", yamlPath}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}

	if _, err := os.Stat(targetPath + ".bak"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no backup, got err=%v", err)
	}
	if data, err := os.ReadFile(targetPath); err != nil || string(data) != "forced overwrite" {
		t.Fatalf("expected overwritten data, got %q err=%v", data, err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunRepoAdd_Success(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	manifest := filepath.Join(dir, "manifest.yml")
	content := "repositories: []\n"
	if err := os.WriteFile(manifest, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"repo", "add", manifest}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}

	registryPath := filepath.Join(home, "registry.json")
	store, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(store.Entries) != 1 {
		t.Fatalf("expected 1 registry entry, got %d", len(store.Entries))
	}

	entry := store.Entries[0]
	if entry.Source != manifest {
		t.Fatalf("expected source %q, got %q", manifest, entry.Source)
	}
	if entry.LocalPath == "" {
		t.Fatalf("expected local path to be recorded")
	}
	data, err := os.ReadFile(entry.LocalPath)
	if err != nil {
		t.Fatalf("failed to read stored manifest: %v", err)
	}
	if string(data) != content {
		t.Fatalf("unexpected stored manifest content: %q", string(data))
	}
	if entry.Digest == "" {
		t.Fatalf("expected digest to be recorded")
	}
	if !strings.Contains(stdout.String(), "registered manifest") {
		t.Fatalf("expected success message in stdout, got %q", stdout.String())
	}
}

func TestRunRepoLs_NoEntries(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"repo", "ls"}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no manifests registered") {
		t.Fatalf("expected empty registry message, got %q", stdout.String())
	}
}

func TestRunRepoLs_ListEntries(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	registryPath := filepath.Join(home, "registry.json")
	entries := registry.Store{
		Entries: []registry.Entry{
			{
				ID:        "aaaa1111",
				Source:    "https://example.com/a.yml",
				LocalPath: filepath.Join(home, "manifests", "a.yml"),
				Digest:    "digest-a",
				UpdatedAt: time.Now().UTC().Add(-time.Hour),
			},
			{
				ID:        "bbbb2222",
				Source:    "https://example.com/b.yml",
				LocalPath: filepath.Join(home, "manifests", "b.yml"),
				Digest:    "digest-b",
				UpdatedAt: time.Now().UTC(),
			},
		},
	}
	if err := entries.Save(registryPath); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"repo", "ls"}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}
	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected multiple lines in stdout, got %q", output)
	}
	if !strings.Contains(lines[0], "ID") || !strings.Contains(lines[0], "SOURCE") || !strings.Contains(lines[0], "UPDATED AT") {
		t.Fatalf("expected header line, got %q", lines[0])
	}
	first := strings.Index(output, "bbbb2222")
	second := strings.Index(output, "aaaa1111")
	if first == -1 || second == -1 || first > second {
		t.Fatalf("expected newer entry first, got output: %q", output)
	}
}

func TestRunRepoRm_ByID(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	manifestPath := filepath.Join(home, "manifests", "abc.yml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("failed to create manifests dir: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte("data"), 0o600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	entry := registry.Entry{
		ID:        "deadbeef",
		Source:    "/tmp/source.yml",
		LocalPath: manifestPath,
		Digest:    "digest",
		UpdatedAt: time.Now().UTC(),
	}
	store := registry.Store{Entries: []registry.Entry{entry}}
	registryPath := filepath.Join(home, "registry.json")
	if err := store.Save(registryPath); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"repo", "rm", entry.ID}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "removed manifest") {
		t.Fatalf("expected removal message, got %q", stdout.String())
	}
	if _, err := os.Stat(manifestPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected manifest to be deleted, stat err=%v", err)
	}
	updated, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if len(updated.Entries) != 0 {
		t.Fatalf("expected registry to be empty, got %d entries", len(updated.Entries))
	}
}

func TestRunRepoRm_NotFound(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"repo", "rm", "missing"}, &stdout, &stderr, nil)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "no manifest found") {
		t.Fatalf("expected missing message, got %q", stderr.String())
	}
}

func TestRunRepoRm_RemovesDownloadedFiles(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	manifestPath := filepath.Join(home, "manifests", "abc.yml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("failed to create manifests dir: %v", err)
	}
	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}
	content := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: file.bin\n        out_dir: %s\n", outDir)
	if err := os.WriteFile(manifestPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
	downloadedFile := filepath.Join(outDir, "file.bin")
	if err := os.WriteFile(downloadedFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write downloaded file: %v", err)
	}

	entry := registry.Entry{
		ID:        "deadbeef",
		Source:    "/tmp/source.yml",
		LocalPath: manifestPath,
		Digest:    "digest",
		UpdatedAt: time.Now().UTC(),
	}
	store := registry.Store{Entries: []registry.Entry{entry}}
	registryPath := filepath.Join(home, "registry.json")
	if err := store.Save(registryPath); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"repo", "rm", entry.ID}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}
	if _, err := os.Stat(downloadedFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected downloaded file to be removed, err=%v", err)
	}
}

func TestRunRepoRm_BackupsModifiedFiles(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	manifestPath := filepath.Join(home, "manifests", "abc.yml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("failed to create manifests dir: %v", err)
	}
	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}
	expected := []byte("expected")
	hasher := blake3.New()
	hasher.Write(expected)
	digest := hex.EncodeToString(hasher.Sum(nil))
	content := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: file.bin\n        out_dir: %s\n        digest: %s\n", outDir, digest)
	if err := os.WriteFile(manifestPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
	targetFile := filepath.Join(outDir, "file.bin")
	original := []byte("user override")
	if err := os.WriteFile(targetFile, original, 0o644); err != nil {
		t.Fatalf("failed to write downloaded file: %v", err)
	}

	entry := registry.Entry{
		ID:        "deadbeef",
		Source:    "/tmp/source.yml",
		LocalPath: manifestPath,
		Digest:    "digest",
		UpdatedAt: time.Now().UTC(),
	}
	store := registry.Store{Entries: []registry.Entry{entry}}
	registryPath := filepath.Join(home, "registry.json")
	if err := store.Save(registryPath); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"repo", "rm", entry.ID}, &stdout, &stderr, nil)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}

	if _, err := os.Stat(targetFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected original file to be moved away, err=%v", err)
	}
	backupPath := targetFile + ".bak"
	if data, err := os.ReadFile(backupPath); err != nil || !bytes.Equal(data, original) {
		t.Fatalf("expected backup to contain original data, got %q err=%v", data, err)
	}
	if !strings.Contains(stderr.String(), "backed up") {
		t.Fatalf("expected backup notice, got %q", stderr.String())
	}
}

func TestRunPkg_RequireSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"pkg"}, &stdout, &stderr, nil)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "require pkg subcommand") {
		t.Fatalf("expected pkg subcommand message, got %q", stderr.String())
	}
}

func TestRunPkgUp_NoEntries(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"pkg", "up"}, &stdout, &stderr, func(string, string) (int64, error) {
		return 0, nil
	})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no manifests registered") {
		t.Fatalf("expected no manifests message, got %q", stdout.String())
	}
}

func TestRunPkgUp_RefreshAndDownload(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	sourceManifest := filepath.Join(dir, "source.yml")
	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}
	newContent := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: tool.bin\n        out_dir: %s\n", outDir)
	if err := os.WriteFile(sourceManifest, []byte(newContent), 0o644); err != nil {
		t.Fatalf("failed to write source manifest: %v", err)
	}

	manifestPath := filepath.Join(home, "manifests", "cached.yml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("failed to create manifest dir: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte("repositories: []\n"), 0o600); err != nil {
		t.Fatalf("failed to write cached manifest: %v", err)
	}

	store := registry.Store{
		Entries: []registry.Entry{
			{
				ID:        "entry",
				Source:    sourceManifest,
				LocalPath: manifestPath,
				Digest:    "old",
				UpdatedAt: time.Now().UTC(),
			},
		},
	}
	registryPath := filepath.Join(home, "registry.json")
	if err := store.Save(registryPath); err != nil {
		t.Fatalf("failed to seed registry: %v", err)
	}

	downloadedPath := filepath.Join(outDir, "tool.bin")
	var downloaded bool
	downloader := func(url, path string) (int64, error) {
		if url != "https://example.com/tool.bin" {
			t.Fatalf("unexpected url %q", url)
		}
		expectTempDownloadPath(t, path)
		if err := os.WriteFile(path, []byte("tool-data"), 0o644); err != nil {
			return 0, err
		}
		downloaded = true
		return 9, nil
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"pkg", "up"}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}
	if !downloaded {
		t.Fatalf("expected download to run")
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	if string(data) != newContent {
		t.Fatalf("expected manifest to be refreshed, got %q", string(data))
	}
	if _, err := os.Stat(downloadedPath); err != nil {
		t.Fatalf("expected downloaded file to exist: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	updatedStore, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("failed to reload registry: %v", err)
	}
	if len(updatedStore.Entries) != 1 {
		t.Fatalf("expected single registry entry, got %d", len(updatedStore.Entries))
	}
	expectedDigest := updatedStore.Entries[0].Digest
	match, actual, err := verifyDigest(manifestPath, expectedDigest)
	if err != nil {
		t.Fatalf("failed to verify digest: %v", err)
	}
	if !match {
		t.Fatalf("expected digest %s to match actual %s", expectedDigest, actual)
	}
	if !strings.Contains(stdout.String(), "updated files for") {
		t.Fatalf("expected success message, got %q", stdout.String())
	}
}

func TestRunPkgUp_BackupsModifiedFiles(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}
	desiredContent := []byte("expected-data")
	hasher := blake3.New()
	hasher.Write(desiredContent)
	digest := hex.EncodeToString(hasher.Sum(nil))
	sourceManifest := filepath.Join(dir, "source.yml")
	manifestContent := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: tool.bin\n        out_dir: %s\n        digest: %s\n", outDir, digest)
	if err := os.WriteFile(sourceManifest, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("failed to write source manifest: %v", err)
	}

	manifestPath := filepath.Join(home, "manifests", "cached.yml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("failed to create manifest dir: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte("repositories: []\n"), 0o600); err != nil {
		t.Fatalf("failed to seed cached manifest: %v", err)
	}

	entry := registry.Entry{
		ID:        "entry",
		Source:    sourceManifest,
		LocalPath: manifestPath,
		Digest:    "old",
		UpdatedAt: time.Now().UTC(),
	}
	registryPath := filepath.Join(home, "registry.json")
	if err := (registry.Store{Entries: []registry.Entry{entry}}).Save(registryPath); err != nil {
		t.Fatalf("failed to seed registry: %v", err)
	}

	targetFile := filepath.Join(outDir, "tool.bin")
	original := []byte("user modifications")
	if err := os.WriteFile(targetFile, original, 0o644); err != nil {
		t.Fatalf("failed to write existing file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	downloader := func(url, path string) (int64, error) {
		if err := os.WriteFile(path, desiredContent, 0o644); err != nil {
			return 0, err
		}
		return int64(len(desiredContent)), nil
	}

	exitCode := Run([]string{"pkg", "up"}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}

	backupPath := targetFile + ".bak"
	if data, err := os.ReadFile(backupPath); err != nil || !bytes.Equal(data, original) {
		t.Fatalf("expected backup %s to contain original data, got %q err=%v", backupPath, data, err)
	}
	if data, err := os.ReadFile(targetFile); err != nil || !bytes.Equal(data, desiredContent) {
		t.Fatalf("expected target to contain refreshed data, got %q err=%v", data, err)
	}
	if !strings.Contains(stderr.String(), "backed up") {
		t.Fatalf("expected backup notice in stderr, got %q", stderr.String())
	}
}

func TestRunPkgUp_RefreshWhenFilesDrift(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}
	desiredContent := []byte("expected-data")
	hasher := blake3.New()
	hasher.Write(desiredContent)
	digest := hex.EncodeToString(hasher.Sum(nil))
	sourceManifest := filepath.Join(dir, "source.yml")
	manifestContent := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: tool.bin\n        out_dir: %s\n        digest: %s\n", outDir, digest)
	if err := os.WriteFile(sourceManifest, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("failed to write source manifest: %v", err)
	}

	manifestPath := filepath.Join(home, "manifests", "cached.yml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("failed to create manifest dir: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o600); err != nil {
		t.Fatalf("failed to seed cached manifest: %v", err)
	}
	_, manifestDigest, err := verifyDigest(manifestPath, "")
	if err != nil {
		t.Fatalf("failed to hash manifest: %v", err)
	}

	entry := registry.Entry{
		ID:        "entry",
		Source:    sourceManifest,
		LocalPath: manifestPath,
		Digest:    manifestDigest,
		UpdatedAt: time.Now().UTC(),
	}
	registryPath := filepath.Join(home, "registry.json")
	if err := (registry.Store{Entries: []registry.Entry{entry}}).Save(registryPath); err != nil {
		t.Fatalf("failed to seed registry: %v", err)
	}

	targetFile := filepath.Join(outDir, "tool.bin")
	original := []byte("user modifications")
	if err := os.WriteFile(targetFile, original, 0o644); err != nil {
		t.Fatalf("failed to write existing file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	downloader := func(url, path string) (int64, error) {
		if err := os.WriteFile(path, desiredContent, 0o644); err != nil {
			return 0, err
		}
		return int64(len(desiredContent)), nil
	}

	exitCode := Run([]string{"pkg", "up"}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}

	backupPath := targetFile + ".bak"
	if data, err := os.ReadFile(backupPath); err != nil || !bytes.Equal(data, original) {
		t.Fatalf("expected backup %s to contain original data, got %q err=%v", backupPath, data, err)
	}
	if data, err := os.ReadFile(targetFile); err != nil || !bytes.Equal(data, desiredContent) {
		t.Fatalf("expected target to contain refreshed data, got %q err=%v", data, err)
	}
	if !strings.Contains(stdout.String(), "files drifted") {
		t.Fatalf("expected drift message, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "backed up") {
		t.Fatalf("expected backup notice in stderr, got %q", stderr.String())
	}
}

func TestRunPkgUp_SkipWhenDigestMatches(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	sourceManifest := filepath.Join(dir, "source.yml")
	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}
	content := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: tool.bin\n        out_dir: %s\n", outDir)
	if err := os.WriteFile(sourceManifest, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write source manifest: %v", err)
	}

	manifestPath := filepath.Join(home, "manifests", "cached.yml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("failed to create manifest dir: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write cached manifest: %v", err)
	}
	_, digest, err := verifyDigest(manifestPath, "")
	if err != nil {
		t.Fatalf("failed to hash manifest: %v", err)
	}

	store := registry.Store{
		Entries: []registry.Entry{
			{
				ID:        "entry",
				Source:    sourceManifest,
				LocalPath: manifestPath,
				Digest:    digest,
				UpdatedAt: time.Now().UTC(),
			},
		},
	}
	registryPath := filepath.Join(home, "registry.json")
	if err := store.Save(registryPath); err != nil {
		t.Fatalf("failed to seed registry: %v", err)
	}

	targetFile := filepath.Join(outDir, "tool.bin")
	if err := os.MkdirAll(filepath.Dir(targetFile), 0o755); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}
	if err := os.WriteFile(targetFile, []byte("existing data"), 0o644); err != nil {
		t.Fatalf("failed to write target file: %v", err)
	}

	downloader := func(url, path string) (int64, error) {
		t.Fatalf("unexpected download call: %s -> %s", url, path)
		return 0, nil
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"pkg", "up"}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "manifest unchanged") {
		t.Fatalf("expected unchanged message, got %q", stdout.String())
	}
}

func TestRunPkgUp_RedownloadFlagDownloadsWhenDigestMatches(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	sourceManifest := filepath.Join(dir, "source.yml")
	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}
	content := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: tool.bin\n        out_dir: %s\n", outDir)
	if err := os.WriteFile(sourceManifest, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write source manifest: %v", err)
	}

	manifestPath := filepath.Join(home, "manifests", "cached.yml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("failed to create manifest dir: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write cached manifest: %v", err)
	}
	_, digest, err := verifyDigest(manifestPath, "")
	if err != nil {
		t.Fatalf("failed to hash manifest: %v", err)
	}

	store := registry.Store{
		Entries: []registry.Entry{
			{
				ID:        "entry",
				Source:    sourceManifest,
				LocalPath: manifestPath,
				Digest:    digest,
				UpdatedAt: time.Now().UTC(),
			},
		},
	}
	registryPath := filepath.Join(home, "registry.json")
	if err := store.Save(registryPath); err != nil {
		t.Fatalf("failed to seed registry: %v", err)
	}

	var downloaded bool
	downloader := func(url, path string) (int64, error) {
		if url != "https://example.com/tool.bin" {
			t.Fatalf("unexpected url %q", url)
		}
		expectTempDownloadPath(t, path)
		if err := os.WriteFile(path, []byte("tool-data"), 0o644); err != nil {
			return 0, err
		}
		downloaded = true
		return 0, nil
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"pkg", "up", "--redownload"}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}
	if !downloaded {
		t.Fatalf("expected download to run")
	}
	if !strings.Contains(stdout.String(), "redownload requested") {
		t.Fatalf("expected redownload message, got %q", stdout.String())
	}
}

func TestRunPkgUp_RedownloadWhenNeverUpdated(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	sourceManifest := filepath.Join(dir, "source.yml")
	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}
	content := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: tool.bin\n        out_dir: %s\n", outDir)
	if err := os.WriteFile(sourceManifest, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write source manifest: %v", err)
	}

	manifestPath := filepath.Join(home, "manifests", "cached.yml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("failed to create manifest dir: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write cached manifest: %v", err)
	}
	_, digest, err := verifyDigest(manifestPath, "")
	if err != nil {
		t.Fatalf("failed to hash manifest: %v", err)
	}

	store := registry.Store{
		Entries: []registry.Entry{
			{
				ID:        "entry",
				Source:    sourceManifest,
				LocalPath: manifestPath,
				Digest:    digest,
			},
		},
	}
	registryPath := filepath.Join(home, "registry.json")
	if err := store.Save(registryPath); err != nil {
		t.Fatalf("failed to seed registry: %v", err)
	}

	var downloaded bool
	downloader := func(url, path string) (int64, error) {
		if url != "https://example.com/tool.bin" {
			t.Fatalf("unexpected url %q", url)
		}
		expectTempDownloadPath(t, path)
		if err := os.WriteFile(path, []byte("tool-data"), 0o644); err != nil {
			return 0, err
		}
		downloaded = true
		return 0, nil
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"pkg", "up"}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}
	if !downloaded {
		t.Fatalf("expected download to run")
	}
	updatedStore, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("failed to reload registry: %v", err)
	}
	if len(updatedStore.Entries) != 1 || updatedStore.Entries[0].UpdatedAt.IsZero() {
		t.Fatalf("expected updated timestamp to be recorded")
	}
}

func TestRunPkgUp_RemovesOldYamlOnChange(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	sourceManifest := filepath.Join(dir, "source.yml")
	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}
	newContent := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: pkg-new.yml\n        rename: pkg-new-renamed\n        out_dir: %s\n", outDir)
	if err := os.WriteFile(sourceManifest, []byte(newContent), 0o644); err != nil {
		t.Fatalf("failed to write source manifest: %v", err)
	}

	manifestPath := filepath.Join(home, "manifests", "cached.yml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("failed to create manifest dir: %v", err)
	}
	oldContent := fmt.Sprintf("repositories:\n  - url: https://example.com\n    files:\n      - file_name: pkg-old.yml\n        rename: pkg-old-renamed\n        out_dir: %s\n", outDir)
	if err := os.WriteFile(manifestPath, []byte(oldContent), 0o600); err != nil {
		t.Fatalf("failed to write cached manifest: %v", err)
	}

	oldFile := filepath.Join(outDir, "pkg-old-renamed")
	if err := os.WriteFile(oldFile, []byte("stale"), 0o644); err != nil {
		t.Fatalf("failed to write old package: %v", err)
	}

	store := registry.Store{
		Entries: []registry.Entry{
			{
				ID:        "entry",
				Source:    sourceManifest,
				LocalPath: manifestPath,
				Digest:    "old",
				UpdatedAt: time.Now().UTC(),
			},
		},
	}
	registryPath := filepath.Join(home, "registry.json")
	if err := store.Save(registryPath); err != nil {
		t.Fatalf("failed to seed registry: %v", err)
	}

	newFile := filepath.Join(outDir, "pkg-new-renamed")
	downloader := func(url, path string) (int64, error) {
		if url != "https://example.com/pkg-new.yml" {
			t.Fatalf("unexpected url %q", url)
		}
		expectTempDownloadPath(t, path)
		if err := os.WriteFile(path, []byte("fresh"), 0o644); err != nil {
			return 0, err
		}
		return 6, nil
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"pkg", "up"}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}
	if _, err := os.Stat(oldFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected old YAML to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Fatalf("expected new YAML to exist, got %v", err)
	}
	if data, err := os.ReadFile(manifestPath); err != nil || string(data) != newContent {
		t.Fatalf("manifest not refreshed (err=%v, data=%q)", err, string(data))
	}
}
