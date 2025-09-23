package req

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDownload_FileSize(t *testing.T) {

	tmpFile, err := os.CreateTemp("", "tmpfile")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()
	orgStdout := os.Stdout

	defer func() {
		os.Stdout = orgStdout
	}()
	os.Stdout = nil

	filepath := "../../test/internal/req/dummyfile"
	fixtureData, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	originalClient := downloadClient
	downloadClient = http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode:    http.StatusOK,
				Body:          io.NopCloser(bytes.NewReader(fixtureData)),
				ContentLength: int64(len(fixtureData)),
				Header:        make(http.Header),
			}, nil
		}),
	}
	defer func() {
		downloadClient = originalClient
	}()

	fs, statErr := os.Stat(filepath)
	if statErr != nil {
		t.Fatalf("failed to stat fixture: %v", statErr)
	}
	Download("http://example.com/dummy", tmpFile.Name())

	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if len(data) != int(fs.Size()) {
		t.Errorf("exp is %d != %d", len(data), fs.Size())
	}

}
