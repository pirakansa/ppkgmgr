package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ppkgmgr/internal/data"
	"ppkgmgr/internal/registry"

	"github.com/spf13/cobra"
)

func newPkgCmd(downloader DownloadFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pkg",
		Short: "Operate on packages stored under ~/.ppkgmgr",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "require pkg subcommand")
			return cliError{code: 1}
		},
	}

	cmd.AddCommand(newPkgUpCmd(downloader))
	return cmd
}

func newPkgUpCmd(downloader DownloadFunc) *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Refresh stored manifests and download referenced files",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "pkg up does not accept arguments")
				return cliError{code: 1}
			}
			return handlePkgUp(cmd, downloader)
		},
	}
}

func handlePkgUp(cmd *cobra.Command, downloader DownloadFunc) error {
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	if downloader == nil {
		fmt.Fprintln(stderr, "pkg up requires a downloader")
		return cliError{code: 5}
	}

	root, err := storageDir()
	if err != nil {
		fmt.Fprintf(stderr, "failed to determine storage directory: %v\n", err)
		return cliError{code: 5}
	}

	registryPath := filepath.Join(root, "registry.json")
	store, err := registry.Load(registryPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to load registry: %v\n", err)
		return cliError{code: 5}
	}

	if len(store.Entries) == 0 {
		fmt.Fprintln(stdout, "no manifests registered")
		return nil
	}

	var hadFailure bool

	for i := range store.Entries {
		entry := &store.Entries[i]
		if entry.Source == "" || entry.LocalPath == "" {
			fmt.Fprintf(stderr, "warning: skipping manifest with incomplete metadata: %s\n", displayValue(entry.Source))
			hadFailure = true
			continue
		}

		var previousTargets []string
		if manifestPaths, err := extractManifestTargets(entry.LocalPath); err == nil {
			previousTargets = manifestPaths
		} else if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "warning: failed to parse stored manifest %s: %v\n", displayValue(entry.Source), err)
		}

		changed, err := refreshStoredManifest(entry)
		if err != nil {
			fmt.Fprintf(stderr, "warning: failed to refresh %s: %v\n", displayValue(entry.Source), err)
			hadFailure = true
			continue
		}
		if !changed {
			fmt.Fprintf(stdout, "manifest unchanged: %s\n", displayValue(entry.Source))
			continue
		}
		fmt.Fprintf(stdout, "refreshed manifest: %s\n", displayValue(entry.Source))

		if len(previousTargets) != 0 {
			cleanupOldYAML(previousTargets, stderr)
		}

		if _, err := os.Stat(entry.LocalPath); err != nil {
			fmt.Fprintf(stderr, "warning: manifest unavailable for %s: %v\n", displayValue(entry.Source), err)
			hadFailure = true
			continue
		}

		fd, err := data.Parse(entry.LocalPath)
		if err != nil {
			fmt.Fprintf(stderr, "warning: failed to parse manifest %s: %v\n", displayValue(entry.Source), err)
			hadFailure = true
			continue
		}

		if err := downloadManifestFiles(fd, downloader, stdout, stderr, false); err != nil {
			hadFailure = true
		} else {
			fmt.Fprintf(stdout, "updated files for: %s\n", displayValue(entry.Source))
		}
	}

	if err := store.Save(registryPath); err != nil {
		fmt.Fprintf(stderr, "failed to save registry: %v\n", err)
		return cliError{code: 5}
	}

	if hadFailure {
		return cliError{code: 4}
	}
	return nil
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

	_, computed, err := verifyDigest(entry.LocalPath, "")
	if err != nil {
		return false, err
	}
	changed := !strings.EqualFold(entry.Digest, computed)
	entry.Digest = computed
	if entry.ID == "" {
		entry.ID = generateEntryID(entry.Source)
	}
	return changed, nil
}

func extractManifestTargets(path string) ([]string, error) {
	fd, err := data.Parse(path)
	if err != nil {
		return nil, err
	}
	return manifestOutputPaths(fd)
}

func cleanupOldYAML(paths []string, stderr io.Writer) {
	for _, path := range paths {
		lower := strings.ToLower(path)
		if !(strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml")) {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "warning: failed to remove outdated package %s: %v\n", path, err)
		}
	}
}
