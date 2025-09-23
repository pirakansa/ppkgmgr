package req

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

var downloadClient = http.Client{
	// // proxy is os environment
	// Transport: &http.Transport{
	// 	Proxy:                 http.ProxyURL(proxyUrl),
	// 	ResponseHeaderTimeout: time.Duration(20) * time.Second,
	// 	TLSHandshakeTimeout:   time.Duration(20) * time.Second,
	// 	ExpectContinueTimeout: time.Duration(10) * time.Second,
	// },
	// Timeout: time.Duration(5) * time.Second,
	CheckRedirect: func(r *http.Request, via []*http.Request) error {
		r.URL.Opaque = r.URL.Path
		return nil
	},
}

func Download(url string, path string) int64 {

	file, err := os.Create(path)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Err: %s\n", err.Error())
		return 0
	}

	defer file.Close()

	response, err := downloadClient.Get(url)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Err: %s\n", err.Error())
		return 0
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Err: %s\n", url)
		return 0
	}

	filesize := response.ContentLength
	dlsize, err := io.Copy(file, response.Body)
	if (filesize != -1) && (dlsize != filesize) {
		fmt.Fprintf(os.Stderr, "Truncated: %s\n", url)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Err: %s\n", err.Error())
		return 0
	}

	fmt.Printf("downloaded: %s => %s\n", url, path)

	return dlsize

}
