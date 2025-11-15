package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ppkgmgr/internal/data"
	"ppkgmgr/internal/registry"

	"github.com/spf13/cobra"
)

func newPkgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pkg",
		Short: "Manage stored manifests",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "require pkg subcommand")
			return cliError{code: 1}
		},
	}

	cmd.AddCommand(newPkgAddCmd())
	return cmd
}

func newPkgAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <manifest>",
		Short: "Register a manifest locally",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				fmt.Fprintln(cmd.ErrOrStderr(), "require manifest path or URL argument")
				return cliError{code: 1}
			}
			return handlePkgAdd(cmd, args[0])
		},
	}
}

func handlePkgAdd(cmd *cobra.Command, source string) error {
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
		AddedAt:   time.Now().UTC(),
	})

	if err := store.Save(registryPath); err != nil {
		fmt.Fprintf(stderr, "failed to save registry: %v\n", err)
		return cliError{code: 5}
	}

	fmt.Fprintf(stdout, "registered manifest: %s\n", target)
	return nil
}
