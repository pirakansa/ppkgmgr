package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ppkgmgr/internal/data"
	"ppkgmgr/internal/registry"
	"ppkgmgr/pkg/req"

	"github.com/zeebo/blake3"
)

var (
	Version = "0.0.0"
)

type downloadFunc func(string, string) (int64, error)

func defaultData(val string, def string) string {
	if val == "" {
		return def
	}
	return val
}

func run(args []string, stdout, stderr io.Writer, downloader downloadFunc) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "require subcommand")
		return 1
	}

	switch args[0] {
	case "ver":
		return runVersion(args[1:], stdout, stderr)
	case "dl":
		return runDownload(args[1:], stdout, stderr, downloader)
	case "pkg":
		return runPkg(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand: %s\n", args[0])
		return 1
	}
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ver", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "unexpected arguments")
		return 1
	}
	fmt.Fprintf(stdout, "Version : %s\n", Version)
	return 0
}

func runDownload(args []string, stdout, stderr io.Writer, downloader downloadFunc) int {
	fs := flag.NewFlagSet("dl", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var spider bool
	fs.BoolVar(&spider, "spider", false, "no dl")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	manifestArgs := fs.Args()
	if len(manifestArgs) == 0 {
		fmt.Fprintln(stderr, "require manifest path argument")
		return 1
	}
	if len(manifestArgs) > 1 {
		fmt.Fprintln(stderr, "unexpected arguments")
		return 1
	}

	path := manifestArgs[0]

	if !isRemotePath(path) {
		if _, err := os.Stat(path); err != nil {
			fmt.Fprintln(stderr, "not found path")
			return 2
		}
	}

	fd, err := data.Parse(path)
	if err != nil {
		fmt.Fprintf(stderr, "failed to parse data: %v\n", err)
		return 3
	}

	var downloadErr error
	for _, repo := range fd.Repo {
		for _, fs := range repo.Files {
			dlurl := fmt.Sprintf("%s/%s", repo.Url, fs.FileName)
			outdir := defaultData(fs.OutDir, ".")
			outname := defaultData(fs.Rename, fs.FileName)
			if filepath.IsAbs(outname) {
				outname = strings.TrimPrefix(outname, filepath.VolumeName(outname))
				outname = strings.TrimLeft(outname, "/\\")
			}
			dlpath := filepath.Join(outdir, outname)
			if spider {
				fmt.Fprintf(stdout, "%s   %s\n", dlurl, dlpath)
				continue
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
		return 4
	}

	return 0
}

func runPkg(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "require pkg subcommand")
		return 1
	}

	switch args[0] {
	case "add":
		return runPkgAdd(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown pkg subcommand: %s\n", args[0])
		return 1
	}
}

func runPkgAdd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pkg add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	paths := fs.Args()
	if len(paths) != 1 {
		fmt.Fprintln(stderr, "require manifest path or URL argument")
		return 1
	}
	source := paths[0]

	raw, err := data.LoadRaw(source)
	if err != nil {
		fmt.Fprintf(stderr, "failed to load manifest: %v\n", err)
		return 3
	}

	root, err := storageDir()
	if err != nil {
		fmt.Fprintf(stderr, "failed to determine storage directory: %v\n", err)
		return 5
	}

	manifestDir := filepath.Join(root, "manifests")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "failed to prepare manifest directory: %v\n", err)
		return 5
	}

	target := filepath.Join(manifestDir, backupFileName(source))
	if err := os.WriteFile(target, raw, 0o600); err != nil {
		fmt.Fprintf(stderr, "failed to write backup: %v\n", err)
		return 5
	}

	_, digest, err := verifyDigest(target, "")
	if err != nil {
		fmt.Fprintf(stderr, "failed to compute digest: %v\n", err)
		return 5
	}

	registryPath := filepath.Join(root, "registry.json")
	store, err := registry.Load(registryPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to load registry: %v\n", err)
		return 5
	}

	entryID := generateEntryID(source)
	if existing, ok := store.GetBySource(source); ok && existing.ID != "" {
		entryID = existing.ID
	}

	store.Upsert(registry.Entry{
		ID:        entryID,
		Source:    source,
		LocalPath: target,
		Digest:    digest,
		AddedAt:   time.Now().UTC(),
	})

	if err := store.Save(registryPath); err != nil {
		fmt.Fprintf(stderr, "failed to save registry: %v\n", err)
		return 5
	}

	fmt.Fprintf(stdout, "registered manifest: %s\n", target)
	return 0
}

func main() {
	code := run(os.Args[1:], os.Stdout, os.Stderr, req.Download)
	os.Exit(code)
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
