package data

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSuccess(t *testing.T) {
	fd, err := Parse("../../test/data/testdata.yml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fd.Repo) != 2 {
		t.Fatalf("expected 2 repositories, got %d", len(fd.Repo))
	}

	first := fd.Repo[0]
	if first.Comment != "jpeg" {
		t.Fatalf("unexpected comment: %q", first.Comment)
	}
	if first.Url != "https://picsum.photos/200" {
		t.Fatalf("unexpected url: %q", first.Url)
	}
	if len(first.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(first.Files))
	}
	file := first.Files[0]
	if file.FileName != "200.jpg" || file.OutDir != "./photos" || file.Rename != "" || file.Digest != "" {
		t.Fatalf("unexpected file data: %+v", file)
	}
}

func TestParseMissingFile(t *testing.T) {
	fd, err := Parse("../../test/data/notfound_testdata.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}

	if len(fd.Repo) != 0 {
		t.Fatalf("expected 0 repositories, got %d", len(fd.Repo))
	}
}

func TestParseInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.yml")
	if err := os.WriteFile(path, []byte("invalid: ["), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err := Parse(path)
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}

func TestParseRemoteSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "repositories:\n  - url: https://example.com\n    files:\n      - file_name: remote.txt\n        out_dir: ./remote\n")
	}))
	t.Cleanup(server.Close)

	fd, err := Parse(server.URL + "/config.yml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fd.Repo) != 1 {
		t.Fatalf("expected 1 repository, got %d", len(fd.Repo))
	}
	repo := fd.Repo[0]
	if repo.Url != "https://example.com" {
		t.Fatalf("unexpected repo url: %q", repo.Url)
	}
	if len(repo.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(repo.Files))
	}
	if repo.Files[0].FileName != "remote.txt" {
		t.Fatalf("unexpected file name: %q", repo.Files[0].FileName)
	}
}

func TestParseRemoteUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	_, err := Parse(server.URL + "/missing.yml")
	if err == nil {
		t.Fatal("expected error for unexpected status")
	}
	if !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("expected unexpected status error, got %v", err)
	}
}
