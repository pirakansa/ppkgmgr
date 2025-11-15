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
	"strings"
	"testing"
	"time"

	"ppkgmgr/internal/registry"

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

	exitCode := Run([]string{"dl", yamlPath}, &stdout, &stderr, downloader)
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

	exitCode := Run([]string{"dl", yamlPath}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_RemoteYAML(t *testing.T) {
	server := newLocalHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/config.yml" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, "repositories:\n  - url: https://example.com\n    files:\n      - file_name: remote.txt\n        out_dir: ./out\n")
	}))

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

	exitCode := Run([]string{"dl", server.URL + "/config.yml"}, &stdout, &stderr, downloader)
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
		if path != filepath.Join(dir, "file.txt") {
			t.Fatalf("unexpected path %q", path)
		}
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

	exitCode := Run([]string{"dl", yamlPath}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "warning: digest mismatch") {
		t.Fatalf("expected digest mismatch warning, got %q", stderr.String())
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
		if path != downloadedPath {
			t.Fatalf("unexpected download path %q", path)
		}
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

func TestRunPkgUp_ForceFlagDownloadsWhenDigestMatches(t *testing.T) {
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
		if path != filepath.Join(outDir, "tool.bin") {
			t.Fatalf("unexpected download path %q", path)
		}
		downloaded = true
		return 0, nil
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"pkg", "up", "-f"}, &stdout, &stderr, downloader)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", exitCode, stderr.String())
	}
	if !downloaded {
		t.Fatalf("expected download to run")
	}
	if !strings.Contains(stdout.String(), "forced refresh") {
		t.Fatalf("expected forced refresh message, got %q", stdout.String())
	}
}

func TestRunPkgUp_ForceDownloadWhenNeverUpdated(t *testing.T) {
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
		if path != filepath.Join(outDir, "tool.bin") {
			t.Fatalf("unexpected download path %q", path)
		}
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
		if path != newFile {
			t.Fatalf("unexpected download path %q", path)
		}
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
