package shared

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeebo/blake3"
)

// DefaultData returns def when val is empty.
func DefaultData(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

// ExpandPath expands environment variables within path.
func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	return os.ExpandEnv(path), nil
}

// StorageDir determines the ppkgmgr working directory.
func StorageDir() (string, error) {
	if override := os.Getenv("PPKGMGR_HOME"); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get user home: %w", err)
	}
	return filepath.Join(home, ".ppkgmgr"), nil
}

// IsRemotePath reports whether the provided path is an HTTP(S) URL.
func IsRemotePath(path string) bool {
	u, err := url.Parse(path)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https"
}

// VerifyDigest computes a BLAKE3 digest for the file and compares it to the expected string.
func VerifyDigest(path, expected string) (bool, string, error) {
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

// BackupFileName generates a deterministic filename for storing manifest backups.
func BackupFileName(source string) string {
	base := filepath.Base(source)
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "manifest.yml"
	}
	base = SanitizeFileName(base)
	sum := blake3.Sum256([]byte(source))
	prefix := hex.EncodeToString(sum[:4])
	return fmt.Sprintf("%s_%s", prefix, base)
}

// SanitizeFileName converts arbitrary input into a filesystem-friendly name.
func SanitizeFileName(name string) string {
	var builder strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			strings.ContainsRune("._-", r) {
			builder.WriteRune(r)
			continue
		}
		builder.WriteRune('_')
	}
	result := builder.String()
	if result == "" {
		return "manifest.yml"
	}
	return result
}

// GenerateEntryID derives a stable identifier for a manifest source.
func GenerateEntryID(source string) string {
	sum := blake3.Sum256([]byte(source))
	return hex.EncodeToString(sum[:8])
}

// BackupIfDigestMismatch backs up the file when a digest mismatch indicates
// the user may have modified the contents manually.
func BackupIfDigestMismatch(path, expected string) (string, error) {
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

	match, _, err := VerifyDigest(path, expected)
	if err != nil {
		return "", fmt.Errorf("verify digest: %w", err)
	}
	if match {
		return "", nil
	}

	backupPath, err := NextBackupPath(path)
	if err != nil {
		return "", err
	}
	if err := os.Rename(path, backupPath); err != nil {
		return "", fmt.Errorf("rename backup: %w", err)
	}
	return backupPath, nil
}

// BackupOutputIfExists renames an existing file to a .bak variant so that
// downloads don't clobber user data unless explicitly forced.
func BackupOutputIfExists(path string) (string, error) {
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

	backupPath, err := NextBackupPath(path)
	if err != nil {
		return "", err
	}
	if err := os.Rename(path, backupPath); err != nil {
		return "", fmt.Errorf("rename backup: %w", err)
	}
	return backupPath, nil
}

// NextBackupPath finds an available backup filename.
func NextBackupPath(path string) (string, error) {
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
