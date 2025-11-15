package cli

import (
	"path/filepath"
	"testing"
)

func TestExpandPathEnv(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "out")
	t.Setenv("PPKGMGR_OUT", custom)

	got, err := expandPath("$PPKGMGR_OUT")
	if err != nil {
		t.Fatalf("expandPath returned error: %v", err)
	}
	if got != custom {
		t.Fatalf("expected %q, got %q", custom, got)
	}
}

func TestExpandPathEmpty(t *testing.T) {
	got, err := expandPath("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}
