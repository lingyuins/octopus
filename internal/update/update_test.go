package update

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoRequest_AllowsLargeDownloadWithoutLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("a", maxUpdateAPIResponseBytes+1024))
	}))
	defer server.Close()

	data, err := doRequest(server.URL, false, 0, "")
	if err != nil {
		t.Fatalf("doRequest() unexpected error: %v", err)
	}

	if got, want := len(data), maxUpdateAPIResponseBytes+1024; got != want {
		t.Fatalf("download size = %d, want %d", got, want)
	}
}

func TestDoRequest_RejectsOversizedAPIResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("b", maxUpdateAPIResponseBytes+1))
	}))
	defer server.Close()

	_, err := doRequest(server.URL, false, maxUpdateAPIResponseBytes, "update API response")
	if err == nil {
		t.Fatal("doRequest() error = nil, want oversized response error")
	}

	want := fmt.Sprintf("update API response exceeds %d bytes limit", maxUpdateAPIResponseBytes)
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}
