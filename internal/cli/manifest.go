package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pirakansa/ppkgmgr/internal/data"
)

// manifestTarget represents a file path generated from a manifest entry along
// with metadata required for cleanup decisions.
type manifestTarget struct {
	path   string
	digest string
}

// downloadManifestFiles walks through every file defined in the manifest and
// downloads them using the provided downloader. When spider is true, only the
// planned download operations are printed. When forceOverwrite is true but
// safeguardForced is also true, digest-protected files are backed up before
// overwriting to preserve user changes.
func downloadManifestFiles(fd data.FileData, downloader DownloadFunc, stdout, stderr io.Writer, spider, forceOverwrite, safeguardForced bool) error {
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

			if !forceOverwrite {
				if backupPath, err := backupOutputIfExists(dlpath); err != nil {
					fmt.Fprintf(stderr, "failed to backup %s: %v\n", dlpath, err)
					return cliError{code: 3}
				} else if backupPath != "" {
					fmt.Fprintf(stderr, "backed up %s to %s\n", dlpath, backupPath)
				}
			} else if safeguardForced && strings.TrimSpace(fs.Digest) != "" {
				if backupPath, err := backupIfDigestMismatch(dlpath, fs.Digest); err != nil {
					if !errors.Is(err, os.ErrNotExist) {
						fmt.Fprintf(stderr, "failed to verify existing %s: %v\n", dlpath, err)
						return cliError{code: 3}
					}
				} else if backupPath != "" {
					fmt.Fprintf(stderr, "backed up %s to %s\n", dlpath, backupPath)
				}
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
			targets = append(targets, manifestTarget{
				path:   path,
				digest: strings.TrimSpace(fs.Digest),
			})
		}
	}
	return targets, nil
}

// backupIfDigestMismatch backs up the file when a digest mismatch indicates
// the user may have modified the contents manually.
func backupIfDigestMismatch(path, expected string) (string, error) {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return "", nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("stat existing file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("existing path %s is a directory", path)
	}

	match, _, err := verifyDigest(path, expected)
	if err != nil {
		return "", fmt.Errorf("verify digest: %w", err)
	}
	if match {
		return "", nil
	}

	backupPath, err := nextBackupPath(path)
	if err != nil {
		return "", err
	}
	if err := os.Rename(path, backupPath); err != nil {
		return "", fmt.Errorf("rename backup: %w", err)
	}
	return backupPath, nil
}

// backupOutputIfExists renames an existing file to a .bak variant so that
// downloads don't clobber user data unless explicitly forced.
func backupOutputIfExists(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("stat existing file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("existing path %s is a directory", path)
	}

	backupPath, err := nextBackupPath(path)
	if err != nil {
		return "", err
	}
	if err := os.Rename(path, backupPath); err != nil {
		return "", fmt.Errorf("rename backup: %w", err)
	}
	return backupPath, nil
}

func nextBackupPath(path string) (string, error) {
	candidate := path + ".bak"
	if _, err := os.Stat(candidate); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
		return "", fmt.Errorf("stat backup candidate: %w", err)
	}
	for i := 1; i < 1000; i++ {
		candidate = fmt.Sprintf("%s.bak.%d", path, i)
		if _, err := os.Stat(candidate); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return candidate, nil
			}
			return "", fmt.Errorf("stat backup candidate: %w", err)
		}
	}
	return "", fmt.Errorf("unable to determine backup name for %s", path)
}
