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

	"github.com/pirakansa/ppkgmgr/internal/cli/shared"
	"github.com/pirakansa/ppkgmgr/internal/registry"
	"github.com/zeebo/blake3"
)

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
	match, actual, err := shared.VerifyDigest(manifestPath, expectedDigest)
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
	_, manifestDigest, err := shared.VerifyDigest(manifestPath, "")
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
	_, digest, err := shared.VerifyDigest(manifestPath, "")
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
	_, digest, err := shared.VerifyDigest(manifestPath, "")
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
	_, digest, err := shared.VerifyDigest(manifestPath, "")
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
