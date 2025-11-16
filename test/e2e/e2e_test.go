package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot returns the repository root relative to this package.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	return root
}

// binaryPath verifies that the ppkgmgr host binary exists and returns its path.
func binaryPath(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	path := filepath.Join(root, "bin", "host", "ppkgmgr")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("ppkgmgr binary not found at %s (run `make build` first): %v", path, err)
	}
	return path
}

// runCommand executes the ppkgmgr binary with the provided arguments, returning stdout/stderr.
func runCommand(t *testing.T, env []string, args ...string) (string, string) {
	t.Helper()
	bin := binaryPath(t)
	cmd := exec.Command(bin, args...)
	cmd.Dir = repoRoot(t)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("command %v failed: %v\nstdout=%s\nstderr=%s", args, err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}

func TestDownloadSpiderOutputsPlannedTargets(t *testing.T) {
	env := os.Environ()
	root := repoRoot(t)
	manifest := filepath.Join(root, "test", "data", "testdata.yml")

	stdout, stderr := runCommand(t, env, "dl", "--spider", manifest)
	if stderr != "" {
		t.Fatalf("expected no stderr output, got %q", stderr)
	}

	expectedLines := []string{
		"https://example.com//index.html   " + filepath.Join("tmp.test", "index.html"),
		"https://example.com//index.html   " + filepath.Join("tmp.test", "index1.html"),
		"https://example.com//index.html   " + filepath.Join("tmp.test", "index2.html"),
		"https://example.com//index.html   " + filepath.Join("tmp.test", "index0.html"),
	}
	expected := strings.Join(expectedLines, "\n") + "\n"

	if stdout != expected {
		t.Fatalf("unexpected spider output\nwant:\n%s\ngot:\n%s", expected, stdout)
	}
}

func TestRepoLifecycleCommands(t *testing.T) {
	home := t.TempDir()
	env := append(os.Environ(), "PPKGMGR_HOME="+home)

	manifestDir := t.TempDir()
	manifestPath := filepath.Join(manifestDir, "config.yml")
	content := "version: 2\nrepositories:\n  - url: https://example.com\n    files:\n      - file_name: file.txt\n"
	if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	if _, stderr := runCommand(t, env, "repo", "add", manifestPath); stderr != "" {
		t.Fatalf("unexpected stderr from repo add: %s", stderr)
	}

	manifestsDir := filepath.Join(home, "manifests")
	entries, err := os.ReadDir(manifestsDir)
	if err != nil {
		t.Fatalf("failed to read manifests dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 stored manifest, got %d", len(entries))
	}
	storedPath := filepath.Join(manifestsDir, entries[0].Name())
	storedData, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatalf("failed to read stored manifest: %v", err)
	}
	if string(storedData) != content {
		t.Fatalf("stored manifest mismatch:\nwant:\n%s\n\ngot:\n%s", content, string(storedData))
	}

	registryPath := filepath.Join(home, "registry.json")
	registryData, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("failed to read registry: %v", err)
	}
	var registry struct {
		Entries []struct {
			ID     string `json:"id"`
			Source string `json:"source"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(registryData, &registry); err != nil {
		t.Fatalf("failed to parse registry: %v", err)
	}
	if len(registry.Entries) != 1 {
		t.Fatalf("expected 1 registry entry, got %d", len(registry.Entries))
	}
	entry := registry.Entries[0]
	if entry.ID == "" {
		t.Fatalf("expected registry entry to have ID")
	}
	if entry.Source != manifestPath {
		t.Fatalf("expected registry source %q, got %q", manifestPath, entry.Source)
	}

	stdout, _ := runCommand(t, env, "repo", "ls")
	if !strings.Contains(stdout, entry.ID) {
		t.Fatalf("repo ls output %q missing entry ID %s", stdout, entry.ID)
	}
	if !strings.Contains(stdout, manifestPath) {
		t.Fatalf("repo ls output %q missing manifest path %s", stdout, manifestPath)
	}

	if _, stderr := runCommand(t, env, "repo", "rm", entry.ID); stderr != "" {
		t.Fatalf("unexpected stderr from repo rm: %s", stderr)
	}

	entries, err = os.ReadDir(manifestsDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to read manifests dir after removal: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected manifests dir to be empty after removal, got %d entries", len(entries))
	}

	registryData, err = os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("failed to read registry after removal: %v", err)
	}
	if err := json.Unmarshal(registryData, &registry); err != nil {
		t.Fatalf("failed to parse registry after removal: %v", err)
	}
	if len(registry.Entries) != 0 {
		t.Fatalf("expected registry to be empty after removal, got %d entries", len(registry.Entries))
	}
}
