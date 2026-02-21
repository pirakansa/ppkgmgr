package manifest

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pirakansa/ppkgmgr/internal/cli/shared"
	"github.com/pirakansa/ppkgmgr/internal/data"
	"github.com/pirakansa/ppkgmgr/pkg/req"
)

// Target represents a file path generated from a manifest entry along
// with metadata required for cleanup decisions.
type Target struct {
	Path   string
	Digest string
}

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

			archiveWhole := isArchiveEncoding(fs.Encoding) && strings.TrimSpace(fs.Extract) == ""
			if !archiveWhole {
				if !forceOverwrite {
					if backupPath, err := shared.BackupOutputIfExists(dlpath); err != nil {
						fmt.Fprintf(stderr, "failed to backup %s: %v\n", dlpath, err)
						return shared.Error{Code: 3}
					} else if backupPath != "" {
						fmt.Fprintf(stderr, "backed up %s to %s\n", dlpath, backupPath)
					}
				} else if safeguardForced && strings.TrimSpace(fs.Digest) != "" {
					if backupPath, err := shared.BackupIfDigestMismatch(dlpath, fs.Digest); err != nil {
						if !errors.Is(err, os.ErrNotExist) {
							fmt.Fprintf(stderr, "failed to verify existing %s: %v\n", dlpath, err)
							return shared.Error{Code: 3}
						}
					} else if backupPath != "" {
						fmt.Fprintf(stderr, "backed up %s to %s\n", dlpath, backupPath)
					}
				}
			}

			tmpFile, err := os.CreateTemp("", "ppkgmgr-*")
			if err != nil {
				fmt.Fprintf(stderr, "failed to create temp file for %s: %v\n", fs.FileName, err)
				return shared.Error{Code: 3}
			}
			tmpPath := tmpFile.Name()
			tmpFile.Close()

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

func resolveDisplayPath(fs data.File) (string, error) {
	if isArchiveEncoding(fs.Encoding) && strings.TrimSpace(fs.Extract) == "" {
		outdir := shared.DefaultData(fs.OutDir, ".")
		return shared.ExpandPath(outdir)
	}
	return ResolvePath(fs)
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

func processDownloadedFile(fs data.File, artifactPath, outputPath string) error {
	if strings.TrimSpace(fs.ArtifactDigest) != "" {
		match, actual, err := shared.VerifyDigest(artifactPath, fs.ArtifactDigest)
		if err != nil {
			return fmt.Errorf("verify artifact digest: %w", err)
		}
		if !match {
			return fmt.Errorf("artifact digest mismatch: expected %s, got %s", fs.ArtifactDigest, actual)
		}
	}

	finalPath := outputPath
	if isArchiveEncoding(fs.Encoding) {
		outDir, err := resolveOutDir(fs)
		if err != nil {
			return err
		}
		extractedPath, err := req.ExtractArchive(fs.Encoding, artifactPath, outDir, fs.Extract, fs.Rename)
		if err != nil {
			return fmt.Errorf("decode file: %w", err)
		}
		if extractedPath != "" {
			finalPath = extractedPath
		} else {
			finalPath = ""
		}
	} else {
		if err := req.DecodeFile(fs.Encoding, artifactPath, outputPath); err != nil {
			return fmt.Errorf("decode file: %w", err)
		}
	}

	if strings.TrimSpace(fs.Digest) != "" {
		if finalPath == "" {
			return fmt.Errorf("digest requires extract to target a single output path")
		}
		match, actual, err := shared.VerifyDigest(finalPath, fs.Digest)
		if err != nil {
			return fmt.Errorf("verify digest: %w", err)
		}
		if !match {
			return cleanupOutputFile(finalPath, fmt.Errorf("digest mismatch: expected %s, got %s", fs.Digest, actual))
		}
	}

	if err := applyMode(finalPath, fs.Mode); err != nil {
		return err
	}

	if err := applySymlink(fs); err != nil {
		return err
	}

	return nil
}

func isArchiveEncoding(encoding string) bool {
	switch strings.TrimSpace(strings.ToLower(encoding)) {
	case "tar+gzip", "tar+xz":
		return true
	default:
		return false
	}
}

func resolveOutDir(fs data.File) (string, error) {
	outDir := shared.DefaultData(fs.OutDir, ".")
	expanded, err := shared.ExpandPath(outDir)
	if err != nil {
		return "", fmt.Errorf("expand output directory %q: %w", outDir, err)
	}
	return expanded, nil
}

func applyMode(path, modeValue string) error {
	if path == "" || strings.TrimSpace(modeValue) == "" {
		return nil
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(modeValue), 8, 32)
	if err != nil {
		return fmt.Errorf("invalid mode %q: %w", modeValue, err)
	}
	if err := os.Chmod(path, os.FileMode(parsed)); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}

func applySymlink(fs data.File) error {
	if fs.Symlink == nil {
		return nil
	}
	link, err := shared.ExpandPath(fs.Symlink.Link)
	if err != nil {
		return fmt.Errorf("expand symlink link %q: %w", fs.Symlink.Link, err)
	}
	target, err := shared.ExpandPath(fs.Symlink.Target)
	if err != nil {
		return fmt.Errorf("expand symlink target %q: %w", fs.Symlink.Target, err)
	}
	if strings.TrimSpace(link) == "" {
		return fmt.Errorf("symlink link is required")
	}
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("symlink target is required")
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return fmt.Errorf("create symlink parent directory: %w", err)
	}
	if _, err := os.Lstat(link); err == nil {
		if err := os.Remove(link); err != nil {
			return fmt.Errorf("remove existing symlink path %s: %w", link, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat symlink path %s: %w", link, err)
	}
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("create symlink %s -> %s: %w", link, target, err)
	}
	return nil
}

// cleanupOutputFile removes the partially written output when verification fails.
func cleanupOutputFile(path string, baseErr error) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w (cleanup %s: %v)", baseErr, path, err)
	}
	return baseErr
}
