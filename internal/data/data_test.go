package data

import (
	"testing"
)

func TestDataParser_RepoCnt(t *testing.T) {

	fd := Parse("../../test/data/testdata.yml")

	for _, repo := range fd.Repo {
		for _, fs := range repo.Files {
			if fs.Rename != "" {
				t.Error("exp is nul")
			}
		}
	}
	if len(fd.Repo) != 2 {
		t.Error("exp is 2")
	}

}

func TestDataParser_NotFoundFile(t *testing.T) {

	fd := Parse("../../test/data/notfound_testdata.yml")

	if len(fd.Repo) != 0 {
		t.Error("exp is 0")
	}

}
