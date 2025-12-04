package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
	"github.com/spf13/cobra"
)

// newUtilCmd wires utility helpers under the util subcommand.
func newUtilCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "util",
		Short: "Utility helpers for working with ppkgmgr artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "require subcommand (run 'ppkgmgr help util' for usage)")
			return cliError{code: 1}
		},
	}

	cmd.AddCommand(newZstdCmd())
	return cmd
}

func newZstdCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "zstd <src> <dst>",
		Short: "Compress a file using zstd and print the BLAKE3 digest",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()

			srcPath, err := expandPath(args[0])
			if err != nil {
				fmt.Fprintf(stderr, "failed to expand source path: %v\n", err)
				return cliError{code: 5}
			}
			dstPath, err := expandPath(args[1])
			if err != nil {
				fmt.Fprintf(stderr, "failed to expand destination path: %v\n", err)
				return cliError{code: 5}
			}

			if srcPath == "" || dstPath == "" {
				fmt.Fprintln(stderr, "require source and destination paths")
				return cliError{code: 1}
			}

			srcAbs, err := filepath.Abs(srcPath)
			if err != nil {
				fmt.Fprintf(stderr, "failed to resolve source path: %v\n", err)
				return cliError{code: 5}
			}
			dstAbs, err := filepath.Abs(dstPath)
			if err != nil {
				fmt.Fprintf(stderr, "failed to resolve destination path: %v\n", err)
				return cliError{code: 5}
			}

			if srcAbs == dstAbs {
				fmt.Fprintln(stderr, "source and destination paths must be different")
				return cliError{code: 1}
			}

			srcFile, err := os.Open(srcPath)
			if err != nil {
				fmt.Fprintf(stderr, "failed to open source: %v\n", err)
				return cliError{code: 5}
			}
			defer srcFile.Close()

			if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
				fmt.Fprintf(stderr, "failed to create destination directory: %v\n", err)
				return cliError{code: 5}
			}

			dstFile, err := os.Create(dstPath)
			if err != nil {
				fmt.Fprintf(stderr, "failed to create destination file: %v\n", err)
				return cliError{code: 5}
			}

			encoder, err := zstd.NewWriter(dstFile)
			if err != nil {
				dstFile.Close()
				fmt.Fprintf(stderr, "failed to initialize compressor: %v\n", err)
				return cliError{code: 5}
			}

			if _, err := io.Copy(encoder, srcFile); err != nil {
				encoder.Close()
				dstFile.Close()
				fmt.Fprintf(stderr, "failed to compress file: %v\n", err)
				return cliError{code: 5}
			}

			if err := encoder.Close(); err != nil {
				dstFile.Close()
				fmt.Fprintf(stderr, "failed to finalize compression: %v\n", err)
				return cliError{code: 5}
			}

			if err := dstFile.Close(); err != nil {
				fmt.Fprintf(stderr, "failed to close destination file: %v\n", err)
				return cliError{code: 5}
			}

			_, digest, err := verifyDigest(dstPath, "")
			if err != nil {
				fmt.Fprintf(stderr, "failed to compute digest: %v\n", err)
				return cliError{code: 5}
			}

			fmt.Fprintln(stdout, digest)
			return nil
		},
	}
}
