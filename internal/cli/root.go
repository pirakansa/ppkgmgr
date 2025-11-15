package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newRootCmd creates the root ppkgmgr command and attaches all subcommands.
func newRootCmd(downloader DownloadFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "ppkgmgr",
		Short:         "Private package manager CLI",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "require subcommand (run 'ppkgmgr help' for usage)")
			return cliError{code: 1}
		},
	}

	cmd.AddCommand(newDownloadCmd(downloader))
	cmd.AddCommand(newRepoCmd())
	cmd.AddCommand(newPkgCmd(downloader))
	cmd.AddCommand(newVersionCmd())
	return cmd
}
