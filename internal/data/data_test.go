package data

import (
	"errors"
	"fmt"
	"net"
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

	if len(fd.Repo) != 3 {
		t.Fatalf("expected 3 repositories, got %d", len(fd.Repo))
	}

	first := fd.Repo[0]
	if first.Comment != "test1" {
		t.Fatalf("unexpected comment: %q", first.Comment)
	}
	if first.Url != "https://example.com/" {
		t.Fatalf("unexpected url: %q", first.Url)
	}
	if len(first.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(first.Files))
	}
	file := first.Files[0]
	if file.FileName != "index.html" || file.OutDir != "./tmp.test" || file.Rename != "" || file.Digest != "" {
		t.Fatalf("unexpected file data: %+v", file)
	}

	second := fd.Repo[1]
	if second.Comment != "test2" || second.Url != "https://example.com/" {
		t.Fatalf("unexpected repo data: %+v", second)
	}
	if len(second.Files) != 2 {
		t.Fatalf("expected 2 files for second repo, got %d", len(second.Files))
	}
	file = second.Files[0]
	if file.FileName != "index.html" || file.Rename != "index1.html" || file.OutDir != "./tmp.test" {
		t.Fatalf("unexpected first file data in second repo: %+v", file)
	}
	if file.Digest != "454499efc25b742a1eaa37e1b2ec934638b05cef87b036235c087d54ee5dde59" {
		t.Fatalf("unexpected digest in first file of second repo: %q", file.Digest)
	}
	file = second.Files[1]
	if file.Rename != "index2.html" || file.Digest != strings.Repeat("f", 64) {
		t.Fatalf("unexpected second file data in second repo: %+v", file)
	}

	third := fd.Repo[2]
	if third.Comment != "test3" || len(third.Files) != 1 {
		t.Fatalf("unexpected third repo data: %+v", third)
	}
	file = third.Files[0]
	if file.Rename != "index0.html" || file.FileName != "index.html" || file.OutDir != "./tmp.test" {
		t.Fatalf("unexpected file data in third repo: %+v", file)
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

func newIPv4Server(t *testing.T, handler http.Handler) *httptest.Server {
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

func TestParseRemoteSuccess(t *testing.T) {
	server := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "repositories:\n  - url: https://example.com\n    files:\n      - file_name: remote.txt\n        out_dir: ./remote\n")
	}))

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
	server := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))

	_, err := Parse(server.URL + "/missing.yml")
	if err == nil {
		t.Fatal("expected error for unexpected status")
	}
	if !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("expected unexpected status error, got %v", err)
	}
}
