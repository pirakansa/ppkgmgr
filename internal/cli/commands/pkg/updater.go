package pkg

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pirakansa/ppkgmgr/internal/cli/manifest"
	"github.com/pirakansa/ppkgmgr/internal/cli/shared"
	"github.com/pirakansa/ppkgmgr/internal/data"
	"github.com/pirakansa/ppkgmgr/internal/registry"
)

// pkgUpdater orchestrates refreshing manifests and retrieving referenced files.
type pkgUpdater struct {
	downloader shared.DownloadFunc
	stdout     io.Writer
	stderr     io.Writer
	force      bool
}

func (u *pkgUpdater) run() error {
	if u.downloader == nil {
		fmt.Fprintln(u.stderr, "pkg up requires a downloader")
		return shared.Error{Code: 5}
	}

	root, err := shared.StorageDir()
	if err != nil {
		fmt.Fprintf(u.stderr, "failed to determine storage directory: %v\n", err)
		return shared.Error{Code: 5}
	}

	registryPath := filepath.Join(root, "registry.json")
	store, err := registry.Load(registryPath)
	if err != nil {
		fmt.Fprintf(u.stderr, "failed to load registry: %v\n", err)
		return shared.Error{Code: 5}
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
		return shared.Error{Code: 5}
	}

	if hadFailure {
		return shared.Error{Code: 4}
	}
	return nil
}

func (u *pkgUpdater) updateEntry(entry *registry.Entry) bool {
	if entry.Source == "" || entry.LocalPath == "" {
		fmt.Fprintf(u.stderr, "warning: skipping manifest with incomplete metadata: %s\n", displayValue(entry.Source))
		return true
	}

	var previousTargets []manifest.Target
	if manifestTargets, err := manifest.ExtractTargets(entry.LocalPath); err == nil {
		previousTargets = manifestTargets
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(u.stderr, "warning: failed to parse stored manifest %s: %v\n", displayValue(entry.Source), err)
	}

	changed, err := refreshStoredManifest(entry)
	if err != nil {
		fmt.Fprintf(u.stderr, "warning: failed to refresh %s: %v\n", displayValue(entry.Source), err)
		return true
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

	switch {
	case changed:
		fmt.Fprintf(u.stdout, "refreshed manifest: %s\n", displayValue(entry.Source))
		if len(previousTargets) != 0 {
			manifest.CleanupOldTargets(previousTargets, u.stderr)
		}
	case u.force:
		fmt.Fprintf(u.stdout, "redownload requested: %s\n", displayValue(entry.Source))
	default:
		needsRefresh, err := manifest.FilesNeedRefresh(fd)
		if err != nil {
			fmt.Fprintf(u.stderr, "warning: failed to inspect files for %s: %v\n", displayValue(entry.Source), err)
			return true
		}
		if !needsRefresh {
			fmt.Fprintf(u.stdout, "manifest unchanged: %s\n", displayValue(entry.Source))
			return false
		}
		fmt.Fprintf(u.stdout, "files drifted: %s\n", displayValue(entry.Source))
	}

	if err := manifest.DownloadFiles(fd, u.downloader, u.stdout, u.stderr, false, true, true); err != nil {
		return true
	}

	fmt.Fprintf(u.stdout, "updated files for: %s\n", displayValue(entry.Source))
	return false
}

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

	_, computed, err := shared.VerifyDigest(entry.LocalPath, "")
	if err != nil {
		return false, err
	}
	changed := entry.UpdatedAt.IsZero() || !strings.EqualFold(entry.Digest, computed)
	entry.Digest = computed
	if entry.ID == "" {
		entry.ID = shared.GenerateEntryID(entry.Source)
	}
	if changed {
		entry.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func displayValue(val string) string {
	if strings.TrimSpace(val) == "" {
		return "-"
	}
	return val
}
