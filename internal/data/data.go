package data

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// FileData represents the manifest root document.
type FileData struct {
	Version int            `yaml:"version,omitempty"`
	Repo    []Repositories `yaml:"repositories"`
}

// Repositories describes a manifest repository entry.
type Repositories struct {
	Comment string `yaml:"_comment"`
	Url     string `yaml:"url"`
	Files   []File `yaml:"files"`
}

// File describes a downloadable file entry.
type File struct {
	FileName       string         `yaml:"file_name"`
	Digest         string         `yaml:"digest,omitempty"`
	ArtifactDigest string         `yaml:"artifact_digest,omitempty"`
	Encoding       string         `yaml:"encoding,omitempty"`
	Extract        string         `yaml:"extract,omitempty"`
	Rename         string         `yaml:"rename,omitempty"`
	Mode           string         `yaml:"mode,omitempty"`
	Symlink        *SymlinkConfig `yaml:"symlink,omitempty"`
	OutDir         string         `yaml:"out_dir"`
}

// SymlinkConfig describes symbolic link creation after file placement.
type SymlinkConfig struct {
	Link   string `yaml:"link"`
	Target string `yaml:"target"`
}

// Parse loads and decodes the manifest at the provided path.
func Parse(path string) (FileData, error) {
	var fd FileData

	raw, err := LoadRaw(path)
	if err != nil {
		return fd, err
	}

	if err := yaml.Unmarshal(raw, &fd); err != nil {
		return fd, fmt.Errorf("decode yaml: %w", err)
	}

	if fd.Version == 0 {
		fd.Version = 3
	}

	return fd, nil
}

// loadYAML retrieves manifest contents from the filesystem or a remote source.
func loadYAML(path string) ([]byte, error) {
	if isRemotePath(path) {
		return fetchRemoteYAML(path)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return raw, nil
}

// LoadRaw returns the raw manifest data located at path.
func LoadRaw(path string) ([]byte, error) {
	return loadYAML(path)
}

// isRemotePath reports whether the manifest is hosted at an HTTP(S) URL.
func isRemotePath(path string) bool {
	u, err := url.Parse(path)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https"
}

// fetchRemoteYAML downloads a manifest from a remote server.
func fetchRemoteYAML(path string) ([]byte, error) {
	resp, err := http.Get(path)
	if err != nil {
		return nil, fmt.Errorf("fetch remote yaml: %w", err)
	}
	if resp.Body == nil {
		return nil, fmt.Errorf("fetch remote yaml: empty body")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch remote yaml: unexpected status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fetch remote yaml: read body: %w", err)
	}

	return data, nil
}
