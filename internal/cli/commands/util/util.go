package util

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pirakansa/ppkgmgr/internal/cli/shared"
)

// New wires utility helpers under the util subcommand.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "util",
		Short: "Utility helpers for working with ppkgmgr artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "require subcommand (run 'ppkgmgr help util' for usage)")
			return shared.Error{Code: 1}
		},
	}

	cmd.AddCommand(newDigCmd())
	cmd.AddCommand(newZstdCmd())
	return cmd
}
