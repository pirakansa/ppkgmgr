package cli

import (
	"errors"
	"io"
)

// DownloadFunc downloads the remote file at the first argument into the
// location provided by the second argument.
type DownloadFunc func(string, string) (int64, error)

type cliError struct {
	code int
}

func (e cliError) Error() string {
	return "cli error"
}

// Run executes the ppkgmgr CLI with the provided arguments and writers,
// returning the process exit code.
func Run(args []string, stdout, stderr io.Writer, downloader DownloadFunc) int {
	root := newRootCmd(downloader)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		return exitCode(err)
	}
	return 0
}

func exitCode(err error) int {
	var cliErr cliError
	if errors.As(err, &cliErr) {
		return cliErr.code
	}
	return 1
}
