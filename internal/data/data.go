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

type FileData struct {
	Repo []Repositories `yaml:"repositories"`
}

type Repositories struct {
	Comment string `yaml:"_comment"`
	Url     string `yaml:"url"`
	Files   []File `yaml:"files"`
}

type File struct {
	FileName string `yaml:"file_name"`
	Digest   string `yaml:"digest,omitempty"`
	Rename   string `yaml:"rename,omitempty"`
	OutDir   string `yaml:"out_dir"`
}

func Parse(path string) (FileData, error) {
	var fd FileData

	raw, err := LoadRaw(path)
	if err != nil {
		return fd, err
	}

	if err := yaml.Unmarshal(raw, &fd); err != nil {
		return fd, fmt.Errorf("decode yaml: %w", err)
	}

	return fd, nil
}

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

func LoadRaw(path string) ([]byte, error) {
	return loadYAML(path)
}

func isRemotePath(path string) bool {
	u, err := url.Parse(path)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https"
}

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
