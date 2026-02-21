package req

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// extractArchive extracts a supported archive into dstDir.
// If extractPath is provided, only that file/directory is moved into dstDir,
// optionally renamed when rename is non-empty. The returned string is the
// resulting output path for extractPath mode; for full extraction it returns "".
func extractArchive(encoding, srcPath, dstDir, extractPath, rename string) (string, error) {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return "", fmt.Errorf("create destination directory: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "ppkgmgr-extract-*")
	if err != nil {
		return "", fmt.Errorf("create temp extraction directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	switch strings.TrimSpace(strings.ToLower(encoding)) {
	case "tar+gzip":
		if err := decodeTarGzip(srcPath, tmpDir); err != nil {
			return "", err
		}
	case "tar+xz":
		if err := decodeTarXz(srcPath, tmpDir); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported encoding: %s", encoding)
	}

	cleanExtract := strings.TrimSpace(extractPath)
	if cleanExtract == "" || cleanExtract == "." {
		if err := moveDirectoryContents(tmpDir, dstDir); err != nil {
			return "", fmt.Errorf("move extracted contents: %w", err)
		}
		return "", nil
	}

	cleanedPath, err := safeRelativePath(cleanExtract)
	if err != nil {
		return "", fmt.Errorf("invalid extract path %q: %w", cleanExtract, err)
	}

	sourcePath := filepath.Join(tmpDir, cleanedPath)
	if _, err := os.Stat(sourcePath); err != nil {
		return "", fmt.Errorf("extract path %q not found in archive: %w", cleanExtract, err)
	}

	targetName := filepath.Base(cleanedPath)
	if strings.TrimSpace(rename) != "" {
		targetName = sanitizeOutputName(rename)
	}

	destinationPath := filepath.Join(dstDir, targetName)
	if err := movePath(sourcePath, destinationPath); err != nil {
		return "", fmt.Errorf("move extracted path: %w", err)
	}

	return destinationPath, nil
}

func safeRelativePath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if cleaned == "." {
		return ".", nil
	}
	if filepath.IsAbs(cleaned) {
		return "", errors.New("absolute paths are not allowed")
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", errors.New("path traversal is not allowed")
	}
	return cleaned, nil
}

func sanitizeOutputName(name string) string {
	trimmed := strings.TrimSpace(name)
	if filepath.IsAbs(trimmed) {
		trimmed = strings.TrimPrefix(trimmed, filepath.VolumeName(trimmed))
		trimmed = strings.TrimLeft(trimmed, "/\\")
	}
	return filepath.Clean(trimmed)
}
