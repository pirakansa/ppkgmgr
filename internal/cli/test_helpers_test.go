package cli

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func newLocalHTTPServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skip: failed to listen on loopback: %v", err)
	}
	server := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: handler},
	}
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func expectTempDownloadPath(t *testing.T, path string) {
	t.Helper()
	if !strings.HasPrefix(path, os.TempDir()) {
		t.Fatalf("unexpected download path %q", path)
	}
}
