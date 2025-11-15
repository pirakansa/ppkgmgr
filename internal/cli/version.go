package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "0.0.0"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ver",
		Short: "Show version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "unexpected arguments")
				return cliError{code: 1}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Version : %s\n", Version)
			return nil
		},
	}
}
