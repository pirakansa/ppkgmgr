package download

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pirakansa/ppkgmgr/internal/cli/manifest"
	"github.com/pirakansa/ppkgmgr/internal/cli/shared"
	"github.com/pirakansa/ppkgmgr/internal/data"
)

// New wires the `dl` command that downloads manifest entries.
func New(downloader shared.DownloadFunc) *cobra.Command {
	var spider bool
	var overwrite bool

	cmd := &cobra.Command{
		Use:   "dl <manifest>",
		Short: "Download files defined in a manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()

			if len(args) == 0 {
				fmt.Fprintln(stderr, "require manifest path argument")
				return shared.Error{Code: 1}
			}
			if len(args) > 1 {
				fmt.Fprintln(stderr, "unexpected arguments")
				return shared.Error{Code: 1}
			}

			path := args[0]
			if !shared.IsRemotePath(path) {
				if _, err := os.Stat(path); err != nil {
					fmt.Fprintln(stderr, "not found path")
					return shared.Error{Code: 2}
				}
			}

			fd, err := data.Parse(path)
			if err != nil {
				fmt.Fprintf(stderr, "failed to parse data: %v\n", err)
				return shared.Error{Code: 3}
			}

			return manifest.DownloadFiles(fd, downloader, stdout, stderr, spider, overwrite, false)
		},
	}

	cmd.Flags().BoolVar(&spider, "spider", false, "no dl")
	cmd.Flags().BoolVarP(&overwrite, "overwrite", "o", false, "overwrite existing files without backups")
	return cmd
}
