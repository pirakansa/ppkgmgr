package cli

import (
	"io"

	"github.com/pirakansa/ppkgmgr/internal/cli/shared"
)

// DownloadFunc downloads the remote file at the first argument into the
// location provided by the second argument.
type DownloadFunc = shared.DownloadFunc

type cliError = shared.Error

// Run executes the ppkgmgr CLI with the provided arguments and writers,
// returning the process exit code.
func Run(args []string, stdout, stderr io.Writer, downloader DownloadFunc) int {
	root := newRootCmd(downloader)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		return shared.ExitCode(err)
	}
	return 0
}
