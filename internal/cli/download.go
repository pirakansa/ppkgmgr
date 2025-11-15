package cli

import (
	"fmt"
	"os"

	"ppkgmgr/internal/data"

	"github.com/spf13/cobra"
)

// newDownloadCmd wires the `dl` command that downloads manifest entries.
func newDownloadCmd(downloader DownloadFunc) *cobra.Command {
	var spider bool

	cmd := &cobra.Command{
		Use:   "dl <manifest>",
		Short: "Download files defined in a manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()

			if len(args) == 0 {
				fmt.Fprintln(stderr, "require manifest path argument")
				return cliError{code: 1}
			}
			if len(args) > 1 {
				fmt.Fprintln(stderr, "unexpected arguments")
				return cliError{code: 1}
			}

			path := args[0]
			if !isRemotePath(path) {
				if _, err := os.Stat(path); err != nil {
					fmt.Fprintln(stderr, "not found path")
					return cliError{code: 2}
				}
			}

			fd, err := data.Parse(path)
			if err != nil {
				fmt.Fprintf(stderr, "failed to parse data: %v\n", err)
				return cliError{code: 3}
			}

			return downloadManifestFiles(fd, downloader, stdout, stderr, spider)
		},
	}

	cmd.Flags().BoolVar(&spider, "spider", false, "no dl")
	return cmd
}
