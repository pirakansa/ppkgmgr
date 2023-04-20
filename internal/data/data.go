package data

import (
	"io/ioutil"

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
	Rename   string `yaml:"rename,omitempty"`
	OutDir   string `yaml:"out_dir"`
}

func Parse(path string) FileData {
	var fd FileData

	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return fd
	}

	err = yaml.Unmarshal(raw, &fd)

	return fd
}
