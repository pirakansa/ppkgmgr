package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"ppkgmgr/internal/data"
	"ppkgmgr/pkg/req"

	"github.com/zeebo/blake3"
)

var (
	Version = "0.0.0"
)

type downloadFunc func(string, string) (int64, error)

func defaultData(val string, def string) string {
	if val == "" {
		return def
	}
	return val
}

func run(args []string, stdout, stderr io.Writer, downloader downloadFunc) int {
	fs := flag.NewFlagSet("ppkgmgr", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var spider bool
	var ver bool
	fs.BoolVar(&spider, "spider", false, "no act")
	fs.BoolVar(&ver, "v", false, "print version")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if ver {
		fmt.Fprintf(stdout, "Version : %s\n", Version)
		return 0
	}

	if len(fs.Args()) < 1 {
		fmt.Fprintln(stderr, "require args")
		return 1
	}

	path := fs.Arg(0)

	if !isRemotePath(path) {
		if _, err := os.Stat(path); err != nil {
			fmt.Fprintln(stderr, "not found path")
			return 2
		}
	}

	fd, err := data.Parse(path)
	if err != nil {
		fmt.Fprintf(stderr, "failed to parse data: %v\n", err)
		return 3
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
		return 4
	}

	return 0
}

func main() {
	code := run(os.Args[1:], os.Stdout, os.Stderr, req.Download)
	os.Exit(code)
}

func isRemotePath(path string) bool {
	u, err := url.Parse(path)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https"
}

func verifyDigest(path, expected string) (bool, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	hasher := blake3.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false, "", fmt.Errorf("hash file: %w", err)
	}

	actual := hasher.Sum(nil)
	actualHex := hex.EncodeToString(actual)

	expected = strings.TrimSpace(expected)
	if expected == "" {
		return true, actualHex, nil
	}

	return strings.EqualFold(expected, actualHex), actualHex, nil
}
