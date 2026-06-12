// Package handler_test covers the ReportHandler HTTP surface.
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"axis-flow-back/internal/middleware"
	report "axis-flow-back/internal/report"
	"axis-flow-back/internal/report/handler"
	"axis-flow-back/internal/report/signing"

	"github.com/go-chi/chi/v5"
)

// ── Fakes ────────────────────────────────────────────────────────────────────

type fakeCache struct {
	data   map[string][]byte
	storeErr error
	fetchErr error
}

func newFakeCache() *fakeCache { return &fakeCache{data: make(map[string][]byte)} }

func (f *fakeCache) Store(_ context.Context, key string, data []byte, _ time.Duration) error {
	if f.storeErr != nil {
		return f.storeErr
	}
	f.data[key] = data
	return nil
}

func (f *fakeCache) Fetch(_ context.Context, key string) ([]byte, error) {
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	d, ok := f.data[key]
	if !ok {
		return nil, report.ErrReportNotFound
	}
	return d, nil
}

type fakePDFGen struct{ err error }

func (f *fakePDFGen) Generate(_ context.Context, _ report.ReportRequest) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []byte("%PDF-1.4 fake"), nil
}

type fakeXLSXGen struct{ err error }

func (f *fakeXLSXGen) Generate(_ context.Context, _ report.ReportRequest) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	// PK zip magic bytes (XLSX header)
	return []byte("PK\x03\x04fake-xlsx"), nil
}

// ── Helper: build a chi router with the handler wired ────────────────────────

const (
	testSigningKey = "test-signing-key-32bytes-longXXXX"
	testTTL        = 3 * time.Minute
)

func buildRouter(c handler.ReportCacheIface, pdf handler.PDFGeneratorIface, xlsx handler.XLSXGeneratorIface) *chi.Mux {
	h := handler.NewReportHandler(c, pdf, xlsx, testSigningKey, testTTL)
	r := chi.NewRouter()
	r.Post("/api/v1/report/run/{type}", h.RunReport)
	r.Get("/api/v1/report/run/{key}", h.DownloadReport)
	return r
}

// injectAuth injects JWT context values the handler needs.
func injectAuth(r *http.Request, userID, tenantID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ContextKeyUserID, userID)
	ctx = context.WithValue(ctx, middleware.ContextKeyTenantID, tenantID)
	return r.WithContext(ctx)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// POST valid pdf → 200 with data.key UUID and data.download_url
func TestRunReport_PDF_200(t *testing.T) {
	c := newFakeCache()
	r := buildRouter(c, &fakePDFGen{}, &fakeXLSXGen{})

	body := `{"filters":{"date_from":"2024-01-01","date_to":"2024-01-31"},"outputFormat":"pdf"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/report/run/sales", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectAuth(req, "user-123", "tenant-abc")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data struct {
			Key         string `json:"key"`
			DownloadURL string `json:"download_url"`
			ExpiresAt   string `json:"expiresAt"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Data.Key == "" {
		t.Error("expected non-empty key")
	}
	if !strings.Contains(resp.Data.DownloadURL, "/api/v1/report/run/") {
		t.Errorf("unexpected download_url: %q", resp.Data.DownloadURL)
	}
	if resp.Data.ExpiresAt == "" {
		t.Error("expected non-empty expires_at")
	}
}

// POST invalid outputFormat → 400
func TestRunReport_InvalidFormat_400(t *testing.T) {
	c := newFakeCache()
	r := buildRouter(c, &fakePDFGen{}, &fakeXLSXGen{})

	body := `{"filters":{},"outputFormat":"csv"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/report/run/sales", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectAuth(req, "user-123", "tenant-abc")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// GET valid sig + cached binary → 200 with correct Content-Type
func TestDownloadReport_ValidSig_200(t *testing.T) {
	c := newFakeCache()
	key := "test-key-abc"
	pdfBytes := []byte("%PDF-1.4 fake content")
	c.data[key] = pdfBytes

	r := buildRouter(c, &fakePDFGen{}, &fakeXLSXGen{})

	expires := fmt.Sprintf("%d", time.Now().Add(testTTL).Unix())
	uid := "user-123"
	sig := signing.Sign(key, expires, uid, testSigningKey)

	url := fmt.Sprintf("/api/v1/report/run/%s?expires=%s&uid=%s&sig=%s", key, expires, uid, sig)
	req := httptest.NewRequest(http.MethodGet, url, nil)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/pdf" {
		t.Errorf("expected application/pdf, got %q", ct)
	}
	if !bytes.Equal(rr.Body.Bytes(), pdfBytes) {
		t.Error("body does not match stored bytes")
	}
}

// GET invalid sig → 403
func TestDownloadReport_InvalidSig_403(t *testing.T) {
	c := newFakeCache()
	r := buildRouter(c, &fakePDFGen{}, &fakeXLSXGen{})

	expires := fmt.Sprintf("%d", time.Now().Add(testTTL).Unix())
	uid := "user-123"
	url := fmt.Sprintf("/api/v1/report/run/some-key?expires=%s&uid=%s&sig=badsig", expires, uid)
	req := httptest.NewRequest(http.MethodGet, url, nil)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

// GET expired → 403
func TestDownloadReport_Expired_403(t *testing.T) {
	c := newFakeCache()
	r := buildRouter(c, &fakePDFGen{}, &fakeXLSXGen{})

	expires := fmt.Sprintf("%d", time.Now().Add(-1*time.Minute).Unix()) // in the past
	uid := "user-123"
	sig := signing.Sign("some-key", expires, uid, testSigningKey)

	url := fmt.Sprintf("/api/v1/report/run/some-key?expires=%s&uid=%s&sig=%s", expires, uid, sig)
	req := httptest.NewRequest(http.MethodGet, url, nil)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

// GET missing key → 404
func TestDownloadReport_MissingKey_404(t *testing.T) {
	c := newFakeCache()
	r := buildRouter(c, &fakePDFGen{}, &fakeXLSXGen{})

	key := "nonexistent-key"
	expires := fmt.Sprintf("%d", time.Now().Add(testTTL).Unix())
	uid := "user-123"
	sig := signing.Sign(key, expires, uid, testSigningKey)

	url := fmt.Sprintf("/api/v1/report/run/%s?expires=%s&uid=%s&sig=%s", key, expires, uid, sig)
	req := httptest.NewRequest(http.MethodGet, url, nil)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
