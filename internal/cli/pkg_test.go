package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupOldTargetsRemovesFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var targets []manifestTarget
	for _, name := range []string{"a.html", "b.yml"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			t.Fatalf("failed to write temp file: %v", err)
		}
		targets = append(targets, manifestTarget{path: path})
	}

	var stderr bytes.Buffer
	cleanupOldTargets(targets, &stderr)

	if stderr.Len() != 0 {
		t.Fatalf("expected no warnings, got %q", stderr.String())
	}

	for _, target := range targets {
		if _, err := os.Stat(target.path); !os.IsNotExist(err) {
			t.Fatalf("expected file %s to be removed, err=%v", target.path, err)
		}
	}
}
