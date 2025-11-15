package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"ppkgmgr/internal/data"
	"ppkgmgr/internal/registry"

	"github.com/spf13/cobra"
)

func newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage stored manifests",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "require repo subcommand")
			return cliError{code: 1}
		},
	}

	cmd.AddCommand(newRepoAddCmd())
	cmd.AddCommand(newRepoLsCmd())
	cmd.AddCommand(newRepoRmCmd())
	return cmd
}

func newRepoAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <manifest>",
		Short: "Register a manifest locally",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				fmt.Fprintln(cmd.ErrOrStderr(), "require manifest path or URL argument")
				return cliError{code: 1}
			}
			return handleRepoAdd(cmd, args[0])
		},
	}
}

func newRepoLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List registered manifests",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "repo ls does not accept arguments")
				return cliError{code: 1}
			}
			return handleRepoLs(cmd)
		},
	}
}

func newRepoRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id_or_source>",
		Short: "Remove a registered manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				fmt.Fprintln(cmd.ErrOrStderr(), "require manifest ID or source argument")
				return cliError{code: 1}
			}
			return handleRepoRm(cmd, args[0])
		},
	}
}

func handleRepoAdd(cmd *cobra.Command, source string) error {
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	raw, err := data.LoadRaw(source)
	if err != nil {
		fmt.Fprintf(stderr, "failed to load manifest: %v\n", err)
		return cliError{code: 3}
	}

	root, err := storageDir()
	if err != nil {
		fmt.Fprintf(stderr, "failed to determine storage directory: %v\n", err)
		return cliError{code: 5}
	}

	manifestDir := filepath.Join(root, "manifests")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "failed to prepare manifest directory: %v\n", err)
		return cliError{code: 5}
	}

	target := filepath.Join(manifestDir, backupFileName(source))
	if err := os.WriteFile(target, raw, 0o600); err != nil {
		fmt.Fprintf(stderr, "failed to write backup: %v\n", err)
		return cliError{code: 5}
	}

	_, digest, err := verifyDigest(target, "")
	if err != nil {
		fmt.Fprintf(stderr, "failed to compute digest: %v\n", err)
		return cliError{code: 5}
	}

	registryPath := filepath.Join(root, "registry.json")
	store, err := registry.Load(registryPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to load registry: %v\n", err)
		return cliError{code: 5}
	}

	entryID := generateEntryID(source)
	if existing, ok := store.GetBySource(source); ok && existing.ID != "" {
		entryID = existing.ID
	}

	store.Upsert(registry.Entry{
		ID:        entryID,
		Source:    source,
		LocalPath: target,
		Digest:    digest,
	})

	if err := store.Save(registryPath); err != nil {
		fmt.Fprintf(stderr, "failed to save registry: %v\n", err)
		return cliError{code: 5}
	}

	fmt.Fprintf(stdout, "registered manifest: %s\n", target)
	return nil
}

func handleRepoLs(cmd *cobra.Command) error {
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

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

	entries := append([]registry.Entry(nil), store.Entries...)
	sort.Slice(entries, func(i, j int) bool {
		switch {
		case entries[i].UpdatedAt.Equal(entries[j].UpdatedAt):
			return entries[i].Source < entries[j].Source
		case entries[i].UpdatedAt.IsZero():
			return false
		case entries[j].UpdatedAt.IsZero():
			return true
		default:
			return entries[i].UpdatedAt.After(entries[j].UpdatedAt)
		}
	})

	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tSOURCE\tUPDATED AT")
	for _, entry := range entries {
		fmt.Fprintf(table, "%s\t%s\t%s\n",
			displayValue(entry.ID),
			displayValue(entry.Source),
			formatUpdatedAt(entry.UpdatedAt),
		)
	}
	if err := table.Flush(); err != nil {
		fmt.Fprintf(stderr, "failed to write manifest list: %v\n", err)
		return cliError{code: 5}
	}
	return nil
}

func handleRepoRm(cmd *cobra.Command, selector string) error {
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

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

	entry, ok := removeRegistryEntry(&store, strings.TrimSpace(selector))
	if !ok {
		fmt.Fprintf(stderr, "no manifest found for %q\n", selector)
		return cliError{code: 2}
	}

	if entry.LocalPath != "" {
		if err := os.Remove(entry.LocalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "failed to remove manifest file: %v\n", err)
			return cliError{code: 5}
		}
	}

	if err := store.Save(registryPath); err != nil {
		fmt.Fprintf(stderr, "failed to save registry: %v\n", err)
		return cliError{code: 5}
	}

	fmt.Fprintf(stdout, "removed manifest: %s\n", displayValue(entry.Source))
	return nil
}

func displayValue(val string) string {
	if strings.TrimSpace(val) == "" {
		return "-"
	}
	return val
}

func formatUpdatedAt(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format(time.RFC3339)
}

func removeRegistryEntry(store *registry.Store, selector string) (registry.Entry, bool) {
	if selector == "" {
		return registry.Entry{}, false
	}
	if entry, ok := store.RemoveByID(selector); ok {
		return entry, true
	}
	return store.RemoveBySource(selector)
}
