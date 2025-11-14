package req

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

type fakeHTTPClient struct {
	doFunc func(*http.Request) (*http.Response, error)
}

func (f fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return f.doFunc(req)
}

func silenceStdout(tb testing.TB) func() {
	tb.Helper()
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		tb.Fatalf("open devnull: %v", err)
	}
	original := os.Stdout
	os.Stdout = devNull
	return func() {
		os.Stdout = original
		devNull.Close()
	}
}

func replaceClient(tb testing.TB, client httpClient) func() {
	tb.Helper()
	original := downloadClient
	downloadClient = client
	return func() {
		downloadClient = original
	}
}

func TestDownload_FileSize(t *testing.T) {
	restoreStdout := silenceStdout(t)
	defer restoreStdout()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "tmpfile")

	filepath := "../../test/internal/req/dummyfile"
	fixtureData, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	defer replaceClient(t, fakeHTTPClient{doFunc: func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Body:          io.NopCloser(bytes.NewReader(fixtureData)),
			ContentLength: int64(len(fixtureData)),
			Header:        make(http.Header),
		}, nil
	}})()

	dlsize, err := Download("http://example.com/dummy", tmpFile)
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if len(data) != len(fixtureData) {
		t.Fatalf("expected %d bytes, got %d", len(fixtureData), len(data))
	}
	if int64(len(fixtureData)) != dlsize {
		t.Fatalf("expected download size %d, got %d", len(fixtureData), dlsize)
	}
}

func TestDownload_HTTPError(t *testing.T) {
	restoreStdout := silenceStdout(t)
	defer restoreStdout()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "tmpfile")

	defer replaceClient(t, fakeHTTPClient{doFunc: func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Status:     "404 Not Found",
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
		}, nil
	}})()

	if _, err := Download("http://example.com/missing", tmpFile); err == nil {
		t.Fatal("expected error for non-200 response")
	}
	if _, statErr := os.Stat(tmpFile); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected file to be removed, stat error: %v", statErr)
	}
}

func TestDownload_ClientError(t *testing.T) {
	restoreStdout := silenceStdout(t)
	defer restoreStdout()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "tmpfile")

	expectedErr := errors.New("network error")
	defer replaceClient(t, fakeHTTPClient{doFunc: func(req *http.Request) (*http.Response, error) {
		return nil, expectedErr
	}})()

	if _, err := Download("http://example.com/error", tmpFile); !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if _, statErr := os.Stat(tmpFile); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected file to be removed, stat error: %v", statErr)
	}
}
