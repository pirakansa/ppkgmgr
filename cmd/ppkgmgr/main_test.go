package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ppkgmgr/internal/registry"

	"github.com/zeebo/blake3"
)

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
	exitCode := run([]string{"ver"}, &stdout, &stderr, nil)
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

func TestRun_RequireSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := run([]string{}, &stdout, &stderr, nil)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "run 'ppkgmgr help'") {
		t.Fatalf("expected help suggestion, got %q", stderr.String())
	}
}

func TestRun_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := run([]string{"help"}, &stdout, &stderr, nil)
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

func TestRunPkg_RequireSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := run([]string{"pkg"}, &stdout, &stderr, nil)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "require pkg subcommand") {
		t.Fatalf("expected pkg subcommand message, got %q", stderr.String())
	}
}

func TestRunPkgAdd_RequireArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := run([]string{"pkg", "add"}, &stdout, &stderr, nil)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "require manifest path or URL argument") {
		t.Fatalf("expected manifest argument message, got %q", stderr.String())
	}
}

func TestRun_RequireManifestArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := run([]string{"dl"}, &stdout, &stderr, nil)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "require manifest path argument") {
		t.Fatalf("expected require manifest message, got %q", stderr.String())
	}
}

func TestRun_PathNotFound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := run([]string{"dl", "missing.yml"}, &stdout, &stderr, nil)
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
	exitCode := run([]string{"dl", badFile}, &stdout, &stderr, nil)
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
	exitCode := run([]string{"dl", "--spider", yamlPath}, &stdout, &stderr, nil)
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

func TestRun_DownloadSuccess(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yml")
	content := "repositories:\n  - url: https://example.com\n    files:\n      - file_name: file.txt\n        out_dir: ./out\n        rename: saved.txt\n"
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
		if path != filepath.Join("./out", "saved.txt") {
			t.Fatalf("unexpected path %q", path)
		}
		return 123, nil
	}

	exitCode := run([]string{"dl", yamlPath}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !called {
		t.Fatalf("expected downloader to be called")
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_DownloadAbsoluteRename(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yml")
	content := "repositories:\n  - url: https://example.com\n    files:\n      - file_name: file.txt\n        out_dir: ./out\n        rename: /etc/passwd\n"
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	var stdout, stderr bytes.Buffer
	downloader := func(url, path string) (int64, error) {
		if path != filepath.Join("./out", "etc/passwd") {
			t.Fatalf("unexpected path %q", path)
		}
		return 0, nil
	}

	exitCode := run([]string{"dl", yamlPath}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_RemoteYAML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/config.yml" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, "repositories:\n  - url: https://example.com\n    files:\n      - file_name: remote.txt\n        out_dir: ./out\n")
	}))
	t.Cleanup(server.Close)

	var stdout, stderr bytes.Buffer
	var called bool
	downloader := func(url, path string) (int64, error) {
		called = true
		if url != "https://example.com/remote.txt" {
			t.Fatalf("unexpected url %q", url)
		}
		if path != filepath.Join("./out", "remote.txt") {
			t.Fatalf("unexpected path %q", path)
		}
		return 1, nil
	}

	exitCode := run([]string{"dl", server.URL + "/config.yml"}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !called {
		t.Fatalf("expected downloader to be called")
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
	exitCode := run([]string{"dl", yamlPath}, &stdout, &stderr, downloader)
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
		if path != filepath.Join(dir, "file.txt") {
			t.Fatalf("unexpected path %q", path)
		}
		if err := os.WriteFile(path, fileContent, 0o644); err != nil {
			return 0, err
		}
		return int64(len(fileContent)), nil
	}

	exitCode := run([]string{"dl", yamlPath}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
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
		if path != filepath.Join(dir, "file.txt") {
			t.Fatalf("unexpected path %q", path)
		}
		if err := os.WriteFile(path, fileContent, 0o644); err != nil {
			return 0, err
		}
		return int64(len(fileContent)), nil
	}

	exitCode := run([]string{"dl", yamlPath}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "warning: digest mismatch") {
		t.Fatalf("expected digest mismatch warning, got %q", stderr.String())
	}
}

func TestRunPkgAdd_Success(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".ppkgmgr")
	t.Setenv("PPKGMGR_HOME", home)

	manifest := filepath.Join(dir, "manifest.yml")
	content := "repositories: []\n"
	if err := os.WriteFile(manifest, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := run([]string{"pkg", "add", manifest}, &stdout, &stderr, nil)
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
