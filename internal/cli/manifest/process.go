package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pirakansa/ppkgmgr/internal/cli/shared"
	"github.com/pirakansa/ppkgmgr/internal/data"
	"github.com/pirakansa/ppkgmgr/pkg/req"
)

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

	finalPath, err := decodeToOutput(fs, artifactPath, outputPath)
	if err != nil {
		return err
	}

	if err := verifyDecodedDigest(fs, finalPath); err != nil {
		return err
	}

	if err := applyMode(finalPath, fs.Mode); err != nil {
		return err
	}

	if err := applySymlink(fs); err != nil {
		return err
	}

	return nil
}

func decodeToOutput(fs data.File, artifactPath, outputPath string) (string, error) {
	if isArchiveEncoding(fs.Encoding) {
		outDir, err := resolveOutDir(fs)
		if err != nil {
			return "", err
		}
		extractedPath, err := req.ExtractArchive(fs.Encoding, artifactPath, outDir, fs.Extract, fs.Rename)
		if err != nil {
			return "", fmt.Errorf("decode file: %w", err)
		}
		return extractedPath, nil
	}

	if err := req.DecodeFile(fs.Encoding, artifactPath, outputPath); err != nil {
		return "", fmt.Errorf("decode file: %w", err)
	}

	return outputPath, nil
}

func verifyDecodedDigest(fs data.File, finalPath string) error {
	if strings.TrimSpace(fs.Digest) == "" {
		return nil
	}
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
