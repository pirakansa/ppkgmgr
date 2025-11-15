package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ppkgmgr/internal/data"
	"ppkgmgr/internal/registry"
)

// pkgUpdater orchestrates refreshing manifests and retrieving referenced files.
type pkgUpdater struct {
	downloader DownloadFunc
	stdout     io.Writer
	stderr     io.Writer
	force      bool
}

// run processes all registered manifests and persists any metadata updates.
func (u *pkgUpdater) run() error {
	if u.downloader == nil {
		fmt.Fprintln(u.stderr, "pkg up requires a downloader")
		return cliError{code: 5}
	}

	root, err := storageDir()
	if err != nil {
		fmt.Fprintf(u.stderr, "failed to determine storage directory: %v\n", err)
		return cliError{code: 5}
	}

	registryPath := filepath.Join(root, "registry.json")
	store, err := registry.Load(registryPath)
	if err != nil {
		fmt.Fprintf(u.stderr, "failed to load registry: %v\n", err)
		return cliError{code: 5}
	}

	if len(store.Entries) == 0 {
		fmt.Fprintln(u.stdout, "no manifests registered")
		return nil
	}

	var hadFailure bool
	for i := range store.Entries {
		if u.updateEntry(&store.Entries[i]) {
			hadFailure = true
		}
	}

	if err := store.Save(registryPath); err != nil {
		fmt.Fprintf(u.stderr, "failed to save registry: %v\n", err)
		return cliError{code: 5}
	}

	if hadFailure {
		return cliError{code: 4}
	}
	return nil
}

// updateEntry refreshes and downloads the files for a single manifest entry.
func (u *pkgUpdater) updateEntry(entry *registry.Entry) bool {
	if entry.Source == "" || entry.LocalPath == "" {
		fmt.Fprintf(u.stderr, "warning: skipping manifest with incomplete metadata: %s\n", displayValue(entry.Source))
		return true
	}

	var previousTargets []manifestTarget
	if manifestTargets, err := extractManifestTargets(entry.LocalPath); err == nil {
		previousTargets = manifestTargets
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(u.stderr, "warning: failed to parse stored manifest %s: %v\n", displayValue(entry.Source), err)
	}

	changed, err := refreshStoredManifest(entry)
	if err != nil {
		fmt.Fprintf(u.stderr, "warning: failed to refresh %s: %v\n", displayValue(entry.Source), err)
		return true
	}
	if !changed && !u.force {
		fmt.Fprintf(u.stdout, "manifest unchanged: %s\n", displayValue(entry.Source))
		return false
	}
	if changed {
		fmt.Fprintf(u.stdout, "refreshed manifest: %s\n", displayValue(entry.Source))
		if len(previousTargets) != 0 {
			cleanupOldTargets(previousTargets, u.stderr)
		}
	} else {
		fmt.Fprintf(u.stdout, "forced refresh: %s\n", displayValue(entry.Source))
	}

	if _, err := os.Stat(entry.LocalPath); err != nil {
		fmt.Fprintf(u.stderr, "warning: manifest unavailable for %s: %v\n", displayValue(entry.Source), err)
		return true
	}

	fd, err := data.Parse(entry.LocalPath)
	if err != nil {
		fmt.Fprintf(u.stderr, "warning: failed to parse manifest %s: %v\n", displayValue(entry.Source), err)
		return true
	}

	if err := downloadManifestFiles(fd, u.downloader, u.stdout, u.stderr, false); err != nil {
		return true
	}

	fmt.Fprintf(u.stdout, "updated files for: %s\n", displayValue(entry.Source))
	return false
}

// refreshStoredManifest downloads the source manifest and updates the registry
// metadata with the latest digest and timestamp.
func refreshStoredManifest(entry *registry.Entry) (bool, error) {
	raw, err := data.LoadRaw(entry.Source)
	if err != nil {
		return false, err
	}

	if err := os.MkdirAll(filepath.Dir(entry.LocalPath), 0o755); err != nil {
		return false, err
	}

	if err := os.WriteFile(entry.LocalPath, raw, 0o600); err != nil {
		return false, err
	}

	_, computed, err := verifyDigest(entry.LocalPath, "")
	if err != nil {
		return false, err
	}
	changed := entry.UpdatedAt.IsZero() || !strings.EqualFold(entry.Digest, computed)
	entry.Digest = computed
	if entry.ID == "" {
		entry.ID = generateEntryID(entry.Source)
	}
	if changed {
		entry.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

// extractManifestTargets parses the manifest and collects all output paths.
func extractManifestTargets(path string) ([]manifestTarget, error) {
	fd, err := data.Parse(path)
	if err != nil {
		return nil, err
	}
	return manifestOutputPaths(fd)
}

// cleanupOldTargets removes any outdated files referenced by a manifest.
func cleanupOldTargets(targets []manifestTarget, stderr io.Writer) {
	for _, target := range targets {
		if err := os.Remove(target.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "warning: failed to remove outdated file %s: %v\n", target.path, err)
		}
	}
}
