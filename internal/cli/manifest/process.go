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

func processDownloadedArtifact(fileEntry data.File, artifactPath, plannedPath string) error {
	if strings.TrimSpace(fileEntry.ArtifactDigest) != "" {
		match, actual, err := shared.VerifyDigest(artifactPath, fileEntry.ArtifactDigest)
		if err != nil {
			return fmt.Errorf("verify artifact digest: %w", err)
		}
		if !match {
			return fmt.Errorf("artifact digest mismatch: expected %s, got %s", fileEntry.ArtifactDigest, actual)
		}
	}

	finalPath, err := decodeArtifactToOutput(fileEntry, artifactPath, plannedPath)
	if err != nil {
		return err
	}

	if err := verifyOutputDigest(fileEntry, finalPath); err != nil {
		return err
	}

	if err := applyOutputMode(finalPath, fileEntry.Mode); err != nil {
		return err
	}

	if err := applyOutputSymlink(fileEntry); err != nil {
		return err
	}

	return nil
}

func decodeArtifactToOutput(fileEntry data.File, artifactPath, plannedPath string) (string, error) {
	decodeOptions := req.DecodeArtifactOptions{
		Encoding:   fileEntry.Encoding,
		SourcePath: artifactPath,
		OutputPath: plannedPath,
		Extract:    fileEntry.Extract,
		Rename:     fileEntry.Rename,
	}

	if req.IsArchiveEncoding(fileEntry.Encoding) {
		outDir, err := resolveOutDir(fileEntry)
		if err != nil {
			return "", err
		}
		decodeOptions.OutputDir = outDir
	}

	outputPath, err := req.DecodeArtifact(decodeOptions)
	if err != nil {
		return "", fmt.Errorf("decode file: %w", err)
	}

	return outputPath, nil
}

func verifyOutputDigest(fileEntry data.File, finalPath string) error {
	if strings.TrimSpace(fileEntry.Digest) == "" {
		return nil
	}
	if finalPath == "" {
		return fmt.Errorf("digest requires extract to target a single output path")
	}

	match, actual, err := shared.VerifyDigest(finalPath, fileEntry.Digest)
	if err != nil {
		return fmt.Errorf("verify digest: %w", err)
	}
	if !match {
		return cleanupOutputFile(finalPath, fmt.Errorf("digest mismatch: expected %s, got %s", fileEntry.Digest, actual))
	}

	return nil
}
func resolveOutDir(fileEntry data.File) (string, error) {
	outDir := shared.DefaultData(fileEntry.OutDir, ".")
	expanded, err := shared.ExpandPath(outDir)
	if err != nil {
		return "", fmt.Errorf("expand output directory %q: %w", outDir, err)
	}
	return expanded, nil
}

func applyOutputMode(path, modeValue string) error {
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

func applyOutputSymlink(fileEntry data.File) error {
	if fileEntry.Symlink == nil {
		return nil
	}
	link, err := shared.ExpandPath(fileEntry.Symlink.Link)
	if err != nil {
		return fmt.Errorf("expand symlink link %q: %w", fileEntry.Symlink.Link, err)
	}
	target, err := shared.ExpandPath(fileEntry.Symlink.Target)
	if err != nil {
		return fmt.Errorf("expand symlink target %q: %w", fileEntry.Symlink.Target, err)
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
