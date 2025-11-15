package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set by the main package prior to executing the CLI.
var Version = "0.0.0"

// newVersionCmd reports CLI version details.
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
