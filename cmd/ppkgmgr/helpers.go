package main

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

func defaultData(val string, def string) string {
	if val == "" {
		return def
	}
	return val
}

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

func isRemotePath(path string) bool {
	u, err := url.Parse(path)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https"
}

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

func generateEntryID(source string) string {
	sum := blake3.Sum256([]byte(source))
	return hex.EncodeToString(sum[:8])
}
