package main

import (
	"os"

	"ppkgmgr/internal/cli"
	"ppkgmgr/pkg/req"
)

var (
	Version = "0.0.0"
)

func main() {
	cli.Version = Version
	code := cli.Run(os.Args[1:], os.Stdout, os.Stderr, req.Download)
	os.Exit(code)
}
