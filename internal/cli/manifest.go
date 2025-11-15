package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"ppkgmgr/internal/data"
)

// manifestTarget represents a file path generated from a manifest entry.
type manifestTarget struct {
	path string
}

// downloadManifestFiles walks through every file defined in the manifest and
// downloads them using the provided downloader. When spider is true, only the
// planned download operations are printed.
func downloadManifestFiles(fd data.FileData, downloader DownloadFunc, stdout, stderr io.Writer, spider bool) error {
	if downloader == nil && !spider {
		fmt.Fprintln(stderr, "downloader is required")
		return cliError{code: 5}
	}

	var downloadErr error
	for _, repo := range fd.Repo {
		for _, fs := range repo.Files {
			dlurl := fmt.Sprintf("%s/%s", repo.Url, fs.FileName)
			dlpath, err := resolveDownloadPath(fs)
			if err != nil {
				fmt.Fprintf(stderr, "failed to determine download path for %s: %v\n", fs.FileName, err)
				return cliError{code: 3}
			}
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
}

// resolveDownloadPath returns the output path for a manifest entry, ensuring
// that the resulting path is safe to use on the local filesystem.
func resolveDownloadPath(fs data.File) (string, error) {
	outdir := defaultData(fs.OutDir, ".")
	expandedDir, err := expandPath(outdir)
	if err != nil {
		return "", fmt.Errorf("expand output directory %q: %w", outdir, err)
	}
	outdir = expandedDir
	outname := defaultData(fs.Rename, fs.FileName)
	if filepath.IsAbs(outname) {
		outname = strings.TrimPrefix(outname, filepath.VolumeName(outname))
		outname = strings.TrimLeft(outname, "/\\")
	}
	return filepath.Join(outdir, outname), nil
}

// manifestOutputPaths collects all output paths declared in the manifest.
func manifestOutputPaths(fd data.FileData) ([]manifestTarget, error) {
	var targets []manifestTarget
	for _, repo := range fd.Repo {
		for _, fs := range repo.Files {
			path, err := resolveDownloadPath(fs)
			if err != nil {
				return nil, err
			}
			targets = append(targets, manifestTarget{path: path})
		}
	}
	return targets, nil
}
