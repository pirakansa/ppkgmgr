package manifest

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pirakansa/ppkgmgr/internal/cli/shared"
	"github.com/pirakansa/ppkgmgr/internal/data"
)

// DownloadFiles walks through every file defined in the manifest and
// downloads them using the provided downloader. When spider is true, only the
// planned download operations are printed. When forceOverwrite is true but
// safeguardForced is also true, digest-protected files are backed up before
// overwriting to preserve user changes.
func DownloadFiles(fd data.FileData, downloader shared.DownloadFunc, stdout, stderr io.Writer, spider, forceOverwrite, safeguardForced bool) error {
	if downloader == nil && !spider {
		fmt.Fprintln(stderr, "downloader is required")
		return shared.Error{Code: 5}
	}

	var downloadErr error
	for _, repo := range fd.Repo {
		for _, fs := range repo.Files {
			dlurl := fmt.Sprintf("%s/%s", repo.Url, fs.FileName)
			dlpath, err := resolveDisplayPath(fs)
			if err != nil {
				fmt.Fprintf(stderr, "failed to determine download path for %s: %v\n", fs.FileName, err)
				return shared.Error{Code: 3}
			}
			if spider {
				fmt.Fprintf(stdout, "%s   %s\n", dlurl, dlpath)
				continue
			}

			if err := backupOutputIfNeeded(fs, dlpath, forceOverwrite, safeguardForced, stderr); err != nil {
				return err
			}

			tmpPath, err := createTempDownloadPath(fs.FileName, stderr)
			if err != nil {
				return err
			}

			if _, err := downloader(dlurl, tmpPath); err != nil {
				fmt.Fprintf(stderr, "failed to download %s: %v\n", dlurl, err)
				downloadErr = err
				_ = os.Remove(tmpPath)
				continue
			}

			if err := processDownloadedFile(fs, tmpPath, dlpath); err != nil {
				fmt.Fprintf(stderr, "failed to process %s: %v\n", dlurl, err)
				downloadErr = err
				_ = os.Remove(tmpPath)
				continue
			}

			if err := os.Remove(tmpPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(stderr, "warning: cleanup temp file %s: %v\n", tmpPath, err)
			}
		}
	}

	if downloadErr != nil {
		return shared.Error{Code: 4}
	}
	return nil
}

func resolveDisplayPath(fs data.File) (string, error) {
	if isArchiveEncoding(fs.Encoding) && strings.TrimSpace(fs.Extract) == "" {
		outdir := shared.DefaultData(fs.OutDir, ".")
		return shared.ExpandPath(outdir)
	}
	return ResolvePath(fs)
}

func backupOutputIfNeeded(fs data.File, dlpath string, forceOverwrite, safeguardForced bool, stderr io.Writer) error {
	archiveWhole := isArchiveEncoding(fs.Encoding) && strings.TrimSpace(fs.Extract) == ""
	if archiveWhole {
		return nil
	}

	if !forceOverwrite {
		if backupPath, err := shared.BackupOutputIfExists(dlpath); err != nil {
			fmt.Fprintf(stderr, "failed to backup %s: %v\n", dlpath, err)
			return shared.Error{Code: 3}
		} else if backupPath != "" {
			fmt.Fprintf(stderr, "backed up %s to %s\n", dlpath, backupPath)
		}
		return nil
	}

	if !safeguardForced || strings.TrimSpace(fs.Digest) == "" {
		return nil
	}

	if backupPath, err := shared.BackupIfDigestMismatch(dlpath, fs.Digest); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "failed to verify existing %s: %v\n", dlpath, err)
			return shared.Error{Code: 3}
		}
	} else if backupPath != "" {
		fmt.Fprintf(stderr, "backed up %s to %s\n", dlpath, backupPath)
	}

	return nil
}

func createTempDownloadPath(fileName string, stderr io.Writer) (string, error) {
	tmpFile, err := os.CreateTemp("", "ppkgmgr-*")
	if err != nil {
		fmt.Fprintf(stderr, "failed to create temp file for %s: %v\n", fileName, err)
		return "", shared.Error{Code: 3}
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		fmt.Fprintf(stderr, "failed to close temp file for %s: %v\n", fileName, err)
		return "", shared.Error{Code: 3}
	}
	return tmpPath, nil
}
