package manifest

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pirakansa/ppkgmgr/internal/cli/shared"
	"github.com/pirakansa/ppkgmgr/internal/data"
)

// Target represents a file path generated from a manifest entry along
// with metadata required for cleanup decisions.
type Target struct {
	Path   string
	Digest string
}

// ResolvePath returns the output path for a manifest entry, ensuring
// that the resulting path is safe to use on the local filesystem.
func ResolvePath(fs data.File) (string, error) {
	outdir := shared.DefaultData(fs.OutDir, ".")
	expandedDir, err := shared.ExpandPath(outdir)
	if err != nil {
		return "", fmt.Errorf("expand output directory %q: %w", outdir, err)
	}
	outdir = expandedDir
	outname := shared.DefaultData(fs.Rename, fs.FileName)
	if filepath.IsAbs(outname) {
		outname = strings.TrimPrefix(outname, filepath.VolumeName(outname))
		outname = strings.TrimLeft(outname, "/\\")
	}
	return filepath.Join(outdir, outname), nil
}

// Targets collects all output paths declared in the manifest.
func Targets(fd data.FileData) ([]Target, error) {
	var targets []Target
	for _, repo := range fd.Repo {
		for _, fs := range repo.Files {
			path, err := ResolvePath(fs)
			if err != nil {
				return nil, err
			}
			targets = append(targets, Target{
				Path:   path,
				Digest: strings.TrimSpace(fs.Digest),
			})
		}
	}
	return targets, nil
}

// ExtractTargets parses the manifest and collects all output paths.
func ExtractTargets(path string) ([]Target, error) {
	fd, err := data.Parse(path)
	if err != nil {
		return nil, err
	}
	return Targets(fd)
}

// CleanupOldTargets removes any outdated files referenced by a manifest.
func CleanupOldTargets(targets []Target, stderr io.Writer) {
	for _, target := range targets {
		if backupPath, err := shared.BackupIfDigestMismatch(target.Path, target.Digest); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(stderr, "warning: failed to safeguard %s: %v\n", target.Path, err)
			}
			continue
		} else if backupPath != "" {
			fmt.Fprintf(stderr, "backed up %s to %s\n", target.Path, backupPath)
			continue
		}
		if err := os.Remove(target.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "warning: failed to remove outdated file %s: %v\n", target.Path, err)
		}
	}
}

// FilesNeedRefresh reports whether any manifest target is missing or fails its digest.
func FilesNeedRefresh(fd data.FileData) (bool, error) {
	for _, repo := range fd.Repo {
		for _, fs := range repo.Files {
			path, err := ResolvePath(fs)
			if err != nil {
				return false, fmt.Errorf("resolve output path for %s: %w", fs.FileName, err)
			}
			info, err := os.Stat(path)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return true, nil
				}
				return false, fmt.Errorf("stat %s: %w", path, err)
			}
			if info.IsDir() {
				return true, nil
			}
			digest := strings.TrimSpace(fs.Digest)
			if digest == "" {
				continue
			}
			match, _, err := shared.VerifyDigest(path, digest)
			if err != nil {
				return false, fmt.Errorf("verify digest for %s: %w", path, err)
			}
			if !match {
				return true, nil
			}
		}
	}
	return false, nil
}
