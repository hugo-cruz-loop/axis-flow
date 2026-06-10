package gotenberg_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"axis-flow-back/internal/cursos/gotenberg"
)

func TestMockGotenbergClientReturnsBytes(t *testing.T) {
	client := gotenberg.New("", false)

	pdf, err := client.GeneratePDF(context.Background(), "<html><body>Test</body></html>")
	if err != nil {
		t.Fatalf("MockGotenbergClient.GeneratePDF() unexpected error: %v", err)
	}
	if len(pdf) == 0 {
		t.Error("expected non-empty PDF bytes from mock client")
	}
}

func TestHTTPGotenbergClientPostsToCorrectEndpoint(t *testing.T) {
	var receivedContentType string
	var receivedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("%PDF-1.4 fake"))
	}))
	defer srv.Close()

	client := gotenberg.New(srv.URL, true)

	pdf, err := client.GeneratePDF(context.Background(), "<html><body>Hello</body></html>")
	if err != nil {
		t.Fatalf("HTTPGotenbergClient.GeneratePDF() unexpected error: %v", err)
	}
	if len(pdf) == 0 {
		t.Error("expected non-empty PDF bytes from HTTP client")
	}
	if receivedPath != "/forms/chromium/convert/html" {
		t.Errorf("expected path /forms/chromium/convert/html, got %q", receivedPath)
	}
	if !strings.Contains(receivedContentType, "multipart/form-data") {
		t.Errorf("expected multipart/form-data Content-Type, got %q", receivedContentType)
	}
}

func TestHTTPGotenbergClientPropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := gotenberg.New(srv.URL, true)

	_, err := client.GeneratePDF(context.Background(), "<html></html>")
	if err == nil {
		t.Error("expected error when Gotenberg returns 500, got nil")
	}
}
