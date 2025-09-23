package data

import (
	"errors"
	"os"
	"testing"
)

func TestDataParser_RepoCnt(t *testing.T) {

	fd, err := Parse("../../test/data/testdata.yml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	fd, err := Parse("../../test/data/notfound_testdata.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}

	if len(fd.Repo) != 0 {
		t.Error("exp is 0")
	}

}
