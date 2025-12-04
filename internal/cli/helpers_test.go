package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pirakansa/ppkgmgr/internal/cli/shared"
)

func TestExpandPathEnv(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "out")
	t.Setenv("PPKGMGR_OUT", custom)

	got, err := shared.ExpandPath("$PPKGMGR_OUT")
	if err != nil {
		t.Fatalf("expandPath returned error: %v", err)
	}
	if got != custom {
		t.Fatalf("expected %q, got %q", custom, got)
	}
}

func TestExpandPathEmpty(t *testing.T) {
	got, err := shared.ExpandPath("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestStorageDirOverride(t *testing.T) {
	root := t.TempDir()
	custom := filepath.Join(root, "state")
	t.Setenv("PPKGMGR_HOME", custom)
	t.Setenv("HOME", filepath.Join(root, "home"))

	got, err := shared.StorageDir()
	if err != nil {
		t.Fatalf("storageDir returned error: %v", err)
	}
	if got != custom {
		t.Fatalf("expected %q, got %q", custom, got)
	}
}

func TestStorageDirDefaultsToHome(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("failed to create fake home: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PPKGMGR_HOME", "")

	got, err := shared.StorageDir()
	if err != nil {
		t.Fatalf("storageDir returned error: %v", err)
	}

	want := filepath.Join(home, ".ppkgmgr")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
