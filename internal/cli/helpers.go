package cli

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeebo/blake3"
)

// defaultData returns def when val is empty.
func defaultData(val string, def string) string {
	if val == "" {
		return def
	}
	return val
}

// expandPath expands environment variables within path.
func expandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	return os.ExpandEnv(path), nil
}

// storageDir determines the ppkgmgr working directory.
func storageDir() (string, error) {
	if override := os.Getenv("PPKGMGR_HOME"); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get user home: %w", err)
	}
	return filepath.Join(home, ".ppkgmgr"), nil
}

// isRemotePath reports whether the provided path is an HTTP(S) URL.
func isRemotePath(path string) bool {
	u, err := url.Parse(path)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https"
}

// verifyDigest computes a BLAKE3 digest for the file and compares it to the expected string.
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

// backupFileName generates a deterministic filename for storing manifest backups.
func backupFileName(source string) string {
	base := filepath.Base(source)
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "manifest.yml"
	}
	base = sanitizeFileName(base)
	sum := blake3.Sum256([]byte(source))
	prefix := hex.EncodeToString(sum[:4])
	return fmt.Sprintf("%s_%s", prefix, base)
}

// sanitizeFileName converts arbitrary input into a filesystem-friendly name.
func sanitizeFileName(name string) string {
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

// generateEntryID derives a stable identifier for a manifest source.
func generateEntryID(source string) string {
	sum := blake3.Sum256([]byte(source))
	return hex.EncodeToString(sum[:8])
}
