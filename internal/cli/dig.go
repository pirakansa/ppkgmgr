package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newDigCmd wires the `dig` command for computing BLAKE3 digests.
func newDigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dig <path>",
		Short: "Print the BLAKE3 digest for the specified file",
		RunE: func(cmd *cobra.Command, args []string) error {
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()

			if len(args) == 0 {
				fmt.Fprintln(stderr, "require file path argument")
				return cliError{code: 1}
			}
			if len(args) > 1 {
				fmt.Fprintln(stderr, "unexpected arguments")
				return cliError{code: 1}
			}

			path, err := expandPath(args[0])
			if err != nil {
				fmt.Fprintf(stderr, "failed to expand path: %v\n", err)
				return cliError{code: 5}
			}
			if path == "" {
				fmt.Fprintln(stderr, "require file path argument")
				return cliError{code: 1}
			}

			_, digest, err := verifyDigest(path, "")
			if err != nil {
				fmt.Fprintf(stderr, "failed to compute digest: %v\n", err)
				return cliError{code: 5}
			}

			fmt.Fprintln(stdout, digest)
			return nil
		},
	}
}
