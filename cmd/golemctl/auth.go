package main

import (
	"fmt"
	"net/http"
	"os"
)

// getToken returns the OIDC bearer token from the GOLEMCTL_TOKEN environment variable.
func getToken() string {
	token := os.Getenv("GOLEMCTL_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "golemctl: GOLEMCTL_TOKEN environment variable is not set")
		fmt.Fprintln(os.Stderr, "Set it to your OIDC bearer token:")
		fmt.Fprintln(os.Stderr, "  export GOLEMCTL_TOKEN=<your-token>")
	}
	return token
}

// setAuthHeader sets the Authorization header on the request.
func setAuthHeader(req *http.Request) {
	token := getToken()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// getAdminURL returns the full admin API URL.
func getAdminURL(path string) string {
	addr := os.Getenv("GOLEMCTL_ADDR")
	if addr == "" {
		addr = "http://localhost:8080"
	}
	return addr + path
}

// bodyReader returns an io.Reader from a byte slice.
func bodyReader(data []byte) *byteSliceReader {
	return &byteSliceReader{data: data, pos: 0}
}

type byteSliceReader struct {
	data []byte
	pos  int
}

func (r *byteSliceReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, nil
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *byteSliceReader) Close() error {
	return nil
}

// stringReader is a simple string reader for request bodies.
type stringReader string

func (s stringReader) Read(p []byte) (n int, err error) {
	if len(s) == 0 {
		return 0, nil
	}
	n = copy(p, []byte(s))
	return n, nil
}

func (s stringReader) Close() error {
	return nil
}
