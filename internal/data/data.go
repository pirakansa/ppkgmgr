package data

import (
	"fmt"
	"os"

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

	raw, err := os.ReadFile(path)
	if err != nil {
		return fd, fmt.Errorf("read file: %w", err)
	}

	if err := yaml.Unmarshal(raw, &fd); err != nil {
		return fd, fmt.Errorf("decode yaml: %w", err)
	}

	return fd, nil
}
