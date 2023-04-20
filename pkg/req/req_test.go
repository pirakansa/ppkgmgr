package req

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDownload_FileSize(t *testing.T) {

	tmpFile, _ := ioutil.TempFile("", "tmpfile")
	defer os.Remove(tmpFile.Name())
	orgStdout := os.Stdout

	defer func() {
		os.Stdout = orgStdout
	}()
	os.Stdout = nil

	filepath := "../../test/internal/req/dummyfile"
	tsrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fb, _ := ioutil.ReadFile(filepath)
		w.Write(fb)
	}))
	defer tsrv.Close()

	fs, _ := os.Stat(filepath)
	Download(tsrv.URL, tmpFile.Name())

	data, _ := ioutil.ReadFile(tmpFile.Name())
	if len(data) != int(fs.Size()) {
		t.Errorf("exp is %d != %d", len(data), fs.Size())
	}

}
