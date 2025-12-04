package pkg

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pirakansa/ppkgmgr/internal/cli/shared"
)

// New provides manifest management helpers under the `pkg` namespace.
func New(downloader shared.DownloadFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pkg",
		Short: "Operate on packages stored under ~/.ppkgmgr",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "require pkg subcommand")
			return shared.Error{Code: 1}
		},
	}

	cmd.AddCommand(newPkgUpCmd(downloader))
	return cmd
}

func newPkgUpCmd(downloader shared.DownloadFunc) *cobra.Command {
	var redownload bool
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Refresh stored manifests and download referenced files",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "pkg up does not accept arguments")
				return shared.Error{Code: 1}
			}
			return handlePkgUp(cmd, downloader, redownload)
		},
	}
	cmd.Flags().BoolVarP(&redownload, "redownload", "r", false, "download even if the manifest digest matches (backups still apply)")
	return cmd
}

func handlePkgUp(cmd *cobra.Command, downloader shared.DownloadFunc, redownload bool) error {
	updater := pkgUpdater{
		downloader: downloader,
		stdout:     cmd.OutOrStdout(),
		stderr:     cmd.ErrOrStderr(),
		force:      redownload,
	}
	return updater.run()
}
