package main

import (
	"flag"
	"fmt"
	"os"
	"ppkgmgr/internal/data"
	"ppkgmgr/pkg/req"
)

var (
	Version = "0.0.0"
)

func defaultData(val string, def string) string {
	if val == "" {
		return def
	}
	return val
}

func main() {

	var spider bool
	var ver bool

	flag.BoolVar(&spider, "spider", false, "no act")
	flag.BoolVar(&ver, "v", false, "print version")
	flag.Parse()

	if ver {
		fmt.Printf("Version : %s\n", Version)
		os.Exit(0)
	}

	if len(flag.Args()) < 1 {
		fmt.Fprintln(os.Stderr, "require args")
		os.Exit(1)
	}

	path := flag.Arg(0)

	if _, err := os.Stat(path); err != nil {
		fmt.Fprintln(os.Stderr, "not found path")
		os.Exit(2)
	}

	fd, err := data.Parse(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse data: %v\n", err)
		os.Exit(3)
	}

	for _, repo := range fd.Repo {
		for _, fs := range repo.Files {
			dlurl := fmt.Sprintf("%s/%s", repo.Url, fs.FileName)
			outdir := defaultData(fs.OutDir, ".")
			outname := defaultData(fs.Rename, fs.FileName)
			dlpath := fmt.Sprintf("%s/%s", outdir, outname)
			if spider {
				fmt.Printf("%s   %s\n", dlurl, dlpath)
			} else {
				req.Download(dlurl, dlpath)
			}
		}
	}

}
