package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	downloadcmd "github.com/pirakansa/ppkgmgr/internal/cli/commands/download"
	pkgcmd "github.com/pirakansa/ppkgmgr/internal/cli/commands/pkg"
	repocmd "github.com/pirakansa/ppkgmgr/internal/cli/commands/repo"
	utilcmd "github.com/pirakansa/ppkgmgr/internal/cli/commands/util"
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
			return cliError{Code: 1}
		},
	}

	cmd.AddCommand(downloadcmd.New(downloader))
	cmd.AddCommand(repocmd.New())
	cmd.AddCommand(pkgcmd.New(downloader))
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(utilcmd.New())
	return cmd
}
