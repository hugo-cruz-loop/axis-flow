// Package gotenberg provides a client for the Gotenberg PDF generation service.
//
// This package is intentionally domain-agnostic: it was originally
// introduced by the cursos module (06_Cursos_Service_Spec) and is
// reused by formularios (10_Formularios_Service_Spec). It lives at
// internal/pdf/gotenberg so neither module owns it.
package gotenberg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// GotenbergClient generates a PDF from raw HTML content.
type GotenbergClient interface {
	GeneratePDF(ctx context.Context, htmlContent string) ([]byte, error)
}

// New returns a GotenbergClient.
// When enabled is false a MockGotenbergClient is returned (useful for dev/test
// environments without a running Gotenberg instance).
// When enabled is true an HTTPGotenbergClient is returned that POSTs to
// {endpoint}/forms/chromium/convert/html.
func New(endpoint string, enabled bool) GotenbergClient {
	if !enabled {
		return &MockGotenbergClient{}
	}
	return &HTTPGotenbergClient{endpoint: endpoint}
}

// MockGotenbergClient returns static PDF bytes without making any HTTP call.
// It is safe to use in tests and CI environments.
type MockGotenbergClient struct{}

// GeneratePDF implements GotenbergClient — returns a minimal valid PDF stub.
func (m *MockGotenbergClient) GeneratePDF(_ context.Context, _ string) ([]byte, error) {
	return []byte("%PDF-1.4 mock-pdf"), nil
}

// HTTPGotenbergClient sends HTML to Gotenberg and returns the PDF bytes.
type HTTPGotenbergClient struct {
	endpoint string
}

// GeneratePDF POSTs the HTML content to Gotenberg via a multipart/form-data
// request and returns the resulting PDF bytes.
func (c *HTTPGotenbergClient) GeneratePDF(ctx context.Context, htmlContent string) ([]byte, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("files", "index.html")
	if err != nil {
		return nil, fmt.Errorf("gotenberg: create form file: %w", err)
	}
	if _, err := io.WriteString(part, htmlContent); err != nil {
		return nil, fmt.Errorf("gotenberg: write html: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("gotenberg: close multipart writer: %w", err)
	}

	url := c.endpoint + "/forms/chromium/convert/html"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: build request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gotenberg: unexpected status %d", resp.StatusCode)
	}

	pdf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: read response: %w", err)
	}
	return pdf, nil
}
