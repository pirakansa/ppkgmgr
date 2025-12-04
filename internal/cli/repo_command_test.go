package cli

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pirakansa/ppkgmgr/internal/registry"
	"github.com/zeebo/blake3"
)

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
