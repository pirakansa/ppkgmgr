package data

import (
	"errors"
	"os"
	"path/filepath"
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
	if file.FileName != "200.jpg" || file.OutDir != "./photos" || file.Rename != "" {
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
