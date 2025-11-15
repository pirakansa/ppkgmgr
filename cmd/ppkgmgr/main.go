package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"ppkgmgr/pkg/req"
)

var (
	Version = "0.0.0"
)

type downloadFunc func(string, string) (int64, error)

type cliError struct {
	code int
}

func (e cliError) Error() string {
	return fmt.Sprintf("exit code %d", e.code)
}

func main() {
	code := run(os.Args[1:], os.Stdout, os.Stderr, req.Download)
	os.Exit(code)
}

func run(args []string, stdout, stderr io.Writer, downloader downloadFunc) int {
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
