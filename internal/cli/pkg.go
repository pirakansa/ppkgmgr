package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newPkgCmd provides manifest management helpers under the `pkg` namespace.
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

// newPkgUpCmd installs the `pkg up` subcommand responsible for refreshing manifests.
func newPkgUpCmd(downloader DownloadFunc) *cobra.Command {
	var redownload bool
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Refresh stored manifests and download referenced files",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "pkg up does not accept arguments")
				return cliError{code: 1}
			}
			return handlePkgUp(cmd, downloader, redownload)
		},
	}
	cmd.Flags().BoolVarP(&redownload, "redownload", "r", false, "download even if the manifest digest matches (backups still apply)")
	return cmd
}

// handlePkgUp executes the `pkg up` workflow using the cobra command context.
func handlePkgUp(cmd *cobra.Command, downloader DownloadFunc, redownload bool) error {
	updater := pkgUpdater{
		downloader: downloader,
		stdout:     cmd.OutOrStdout(),
		stderr:     cmd.ErrOrStderr(),
		force:      redownload,
	}
	return updater.run()
}
