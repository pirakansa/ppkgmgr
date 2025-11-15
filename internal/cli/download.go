package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ppkgmgr/internal/data"

	"github.com/spf13/cobra"
)

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

			var downloadErr error
			for _, repo := range fd.Repo {
				for _, fs := range repo.Files {
					dlurl := fmt.Sprintf("%s/%s", repo.Url, fs.FileName)
					outdir := defaultData(fs.OutDir, ".")
					outname := defaultData(fs.Rename, fs.FileName)
					if filepath.IsAbs(outname) {
						outname = strings.TrimPrefix(outname, filepath.VolumeName(outname))
						outname = strings.TrimLeft(outname, "/\\")
					}
					dlpath := filepath.Join(outdir, outname)
					if spider {
						fmt.Fprintf(stdout, "%s   %s\n", dlurl, dlpath)
						continue
					}

					if _, err := downloader(dlurl, dlpath); err != nil {
						fmt.Fprintf(stderr, "failed to download %s: %v\n", dlurl, err)
						downloadErr = err
						continue
					}

					if fs.Digest != "" {
						match, actual, err := verifyDigest(dlpath, fs.Digest)
						if err != nil {
							fmt.Fprintf(stderr, "warning: failed to verify digest for %s: %v\n", dlpath, err)
							continue
						}
						if !match {
							fmt.Fprintf(stderr, "warning: digest mismatch for %s (expected %s, got %s)\n", dlpath, fs.Digest, actual)
						}
					}
				}
			}

			if downloadErr != nil {
				return cliError{code: 4}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&spider, "spider", false, "no dl")
	return cmd
}
