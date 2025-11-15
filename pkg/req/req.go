package req

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var downloadClient httpClient = &http.Client{
	CheckRedirect: func(r *http.Request, via []*http.Request) error {
		r.URL.Opaque = r.URL.Path
		return nil
	},
}

// Download retrieves the content at url and writes it to path, returning the number of bytes copied.
func Download(url string, path string) (int64, error) {
	if err := ensureDir(path); err != nil {
		return 0, err
	}

	file, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create file: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		file.Close()
		return 0, fmt.Errorf("create request: %w", err)
	}

	response, err := downloadClient.Do(req)
	if err != nil {
		file.Close()
		removeOnError(path)
		return 0, fmt.Errorf("execute request: %w", err)
	}
	if response.Body == nil {
		file.Close()
		removeOnError(path)
		return 0, errors.New("empty response body")
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		file.Close()
		removeOnError(path)
		return 0, fmt.Errorf("unexpected status: %s", response.Status)
	}

	filesize := response.ContentLength
	dlsize, err := io.Copy(file, response.Body)
	if err != nil {
		file.Close()
		removeOnError(path)
		return dlsize, fmt.Errorf("copy body: %w", err)
	}

	if closeErr := file.Close(); closeErr != nil {
		removeOnError(path)
		return dlsize, fmt.Errorf("close file: %w", closeErr)
	}

	if (filesize != -1) && (dlsize != filesize) {
		fmt.Fprintf(os.Stderr, "Truncated: %s\n", url)
	}

	fmt.Printf("downloaded: %s => %s\n", url, path)

	return dlsize, nil
}

// ensureDir creates the directory hierarchy required for path.
func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return nil
}

// removeOnError best-effort deletes files created during a failed download.
func removeOnError(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "cleanup failed: %v\n", err)
	}
}
