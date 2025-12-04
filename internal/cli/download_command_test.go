package cli

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zeebo/blake3"
)

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
