// Binary main package for the ppkgmgr command-line application.
package main

import (
	"os"

	"ppkgmgr/internal/cli"
	"ppkgmgr/pkg/req"
)

// Version reports the build-time version string injected by ldflags.
var (
	Version = "0.0.0"
)

func main() {
	cli.Version = Version
	code := cli.Run(os.Args[1:], os.Stdout, os.Stderr, req.Download)
	os.Exit(code)
}
